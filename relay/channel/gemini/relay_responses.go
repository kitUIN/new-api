package gemini

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func GeminiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	logger.LogDebug(c, "Gemini responses response body: %s", responseBody)

	var geminiResponse dto.GeminiChatResponse
	if err := common.Unmarshal(responseBody, &geminiResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	markGeminiGoogleSearchCall(c, &geminiResponse)
	countGeminiBillableFunctionCalls(info, &geminiResponse)
	if len(geminiResponse.Candidates) == 0 {
		usage := buildUsageFromGeminiMetadata(geminiResponse.UsageMetadata, info.GetEstimatePromptTokens())
		if geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
			common.SetContextKey(c, constant.ContextKeyAdminRejectReason, fmt.Sprintf("gemini_block_reason=%s", *geminiResponse.PromptFeedback.BlockReason))
			return &usage, types.NewOpenAIError(
				errors.New("request blocked by Gemini API: "+*geminiResponse.PromptFeedback.BlockReason),
				types.ErrorCodePromptBlocked,
				http.StatusBadRequest,
			)
		}
		common.SetContextKey(c, constant.ContextKeyAdminRejectReason, "gemini_empty_candidates")
		return &usage, types.NewOpenAIError(
			errors.New("empty response from Gemini API"),
			types.ErrorCodeEmptyResponse,
			http.StatusInternalServerError,
		)
	}

	chatResp := responseGeminiChat2OpenAI(c, &geminiResponse)
	chatResp.Model = info.UpstreamModelName
	usage := buildUsageFromGeminiMetadata(geminiResponse.UsageMetadata, info.GetEstimatePromptTokens())
	chatResp.Usage = usage

	responsesResp, responsesUsage, err := openaicompat.ChatCompletionsResponseToResponsesResponse(chatResp, helper.GetResponseID(c))
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if responsesUsage == nil || responsesUsage.TotalTokens == 0 {
		responsesResp.Usage = openaicompat.UsageFromChatUsage(&usage)
	}
	if queries := geminiWebSearchQueries(&geminiResponse); len(queries) > 0 {
		action, marshalErr := common.Marshal(map[string]any{
			"type":    "search",
			"queries": queries,
		})
		if marshalErr != nil {
			return nil, types.NewOpenAIError(marshalErr, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		}
		responsesResp.Output = append(responsesResp.Output, dto.ResponsesOutput{
			Type:   "web_search_call",
			ID:     "ws_" + common.GetUUID(),
			Status: "completed",
			Action: action,
		})
	}

	responseBody, err = common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return &usage, nil
}

func GeminiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	responseID := helper.GetResponseID(c)
	created := common.GetTimestamp()
	state := openaicompat.NewChatToResponsesStreamState(responseID, info.UpstreamModelName)
	state.Created = created
	finishReason := constant.FinishReasonStop
	toolCallIndexByChoice := make(map[int]map[string]int)
	nextToolCallIndexByChoice := make(map[int]int)
	var streamErr *types.NewAPIError
	terminalSeen := false
	terminalFailed := false
	webSearchQueries := make([]string, 0)
	seenWebSearchQueries := make(map[string]struct{})
	groundingState := newGeminiGroundingStreamState()
	partialCallState := newGeminiPartialFunctionCallState()

	sendEvent := func(event openaicompat.ChatToResponsesStreamEvent) bool {
		data, err := common.Marshal(event.Payload)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
			return false
		}
		helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event.Type}, string(data))
		return true
	}
	sendFailure := func(code, message string) {
		if terminalFailed {
			return
		}
		terminalFailed = true
		for _, event := range openaicompat.FailChatCompletionsStreamToResponses(state, code, message, "") {
			if !sendEvent(event) {
				return
			}
		}
	}
	sendChunk := func(chunk *dto.ChatCompletionsStreamResponse) bool {
		events, err := openaicompat.ChatCompletionsStreamChunkToResponsesEvents(chunk, state)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		for _, event := range events {
			if !sendEvent(event) {
				return false
			}
		}
		return true
	}

	usage, err := geminiStreamHandler(c, info, resp, func(data string, geminiResponse *dto.GeminiChatResponse) bool {
		prepared, prepareErr := partialCallState.prepare(geminiResponse)
		if prepareErr != nil {
			streamErr = types.NewOpenAIError(prepareErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			return false
		}
		geminiResponse = prepared
		if len(geminiResponse.Candidates) == 0 && geminiResponse.PromptFeedback != nil && geminiResponse.PromptFeedback.BlockReason != nil {
			reason := *geminiResponse.PromptFeedback.BlockReason
			common.SetContextKey(c, constant.ContextKeyAdminRejectReason, fmt.Sprintf("gemini_block_reason=%s", reason))
			sendFailure(string(types.ErrorCodePromptBlocked), "request blocked by Gemini API: "+reason)
			return false
		}
		for _, candidate := range geminiResponse.Candidates {
			if candidate.FinishReason != nil && strings.TrimSpace(*candidate.FinishReason) != "" {
				terminalSeen = true
				break
			}
		}
		for _, query := range geminiWebSearchQueries(geminiResponse) {
			if _, exists := seenWebSearchQueries[query]; exists {
				continue
			}
			seenWebSearchQueries[query] = struct{}{}
			webSearchQueries = append(webSearchQueries, query)
		}
		response, isStop := streamResponseGeminiChat2OpenAI(geminiResponse)
		groundingState.annotate(geminiResponse, response)
		response.Id = responseID
		response.Created = created
		response.Model = info.UpstreamModelName

		if response.IsToolCall() {
			finishReason = constant.FinishReasonToolCalls
		}
		for choiceIdx := range response.Choices {
			choiceKey := response.Choices[choiceIdx].Index
			for toolIdx := range response.Choices[choiceIdx].Delta.ToolCalls {
				tool := &response.Choices[choiceIdx].Delta.ToolCalls[toolIdx]
				if tool.ID == "" {
					continue
				}
				indexByID := toolCallIndexByChoice[choiceKey]
				if indexByID == nil {
					indexByID = make(map[string]int)
					toolCallIndexByChoice[choiceKey] = indexByID
				}
				if idx, ok := indexByID[tool.ID]; ok {
					tool.SetIndex(idx)
					continue
				}
				idx := nextToolCallIndexByChoice[choiceKey]
				nextToolCallIndexByChoice[choiceKey] = idx + 1
				indexByID[tool.ID] = idx
				tool.SetIndex(idx)
			}
		}

		if !sendChunk(response) {
			return false
		}
		if isStop {
			return sendChunk(helper.GenerateStopResponse(responseID, created, info.UpstreamModelName, finishReason))
		}
		return true
	})
	if terminalFailed {
		return usage, streamErr
	}
	if err != nil {
		sendFailure("server_error", err.Error())
		return usage, streamErr
	}
	if streamErr != nil {
		sendFailure("server_error", streamErr.Error())
		return usage, streamErr
	}
	if info.StreamStatus != nil {
		if info.StreamStatus.EndReason == relaycommon.StreamEndReasonClientGone {
			return usage, nil
		}
		if !info.StreamStatus.IsNormalEnd() || info.StreamStatus.HasErrors() {
			sendFailure("server_error", "Gemini stream ended unexpectedly: "+info.StreamStatus.Summary())
			return usage, streamErr
		}
	}
	if partialErr := partialCallState.validateComplete(); partialErr != nil {
		sendFailure("server_error", partialErr.Error())
		return usage, streamErr
	}
	if !terminalSeen {
		sendFailure("server_error", "Gemini stream ended before a terminal candidate")
		return usage, streamErr
	}
	webSearchEvents, webSearchErr := openaicompat.AppendCompletedWebSearchToResponses(state, webSearchQueries)
	if webSearchErr != nil {
		sendFailure("server_error", webSearchErr.Error())
		return usage, streamErr
	}
	for _, event := range webSearchEvents {
		if !sendEvent(event) {
			return usage, streamErr
		}
	}

	if usage != nil {
		state.Usage = openaicompat.UsageFromChatUsage(usage)
	}
	for _, event := range openaicompat.FinalizeChatCompletionsStreamToResponses(state) {
		if !sendEvent(event) {
			return nil, streamErr
		}
	}
	return usage, nil
}

func geminiWebSearchQueries(response *dto.GeminiChatResponse) []string {
	if response == nil {
		return nil
	}
	queries := make([]string, 0)
	seen := make(map[string]struct{})
	for _, candidate := range response.Candidates {
		if candidate.GroundingMetadata == nil {
			continue
		}
		for _, query := range candidate.GroundingMetadata.WebSearchQueries {
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}
			if _, exists := seen[query]; exists {
				continue
			}
			seen[query] = struct{}{}
			queries = append(queries, query)
		}
	}
	return queries
}
