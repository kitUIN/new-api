package gemini

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamResponseGeminiChat2OpenAIAttachesReportedUsage(t *testing.T) {
	response, isStop := streamResponseGeminiChat2OpenAI(&dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{Content: dto.GeminiChatContent{
			Role:  "model",
			Parts: []dto.GeminiPart{{Text: "hello"}},
		}}},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount: 3868,
			TotalTokenCount:  3868,
		},
	})

	require.False(t, isStop)
	require.NotNil(t, response)
	require.NotNil(t, response.Usage)
	require.Equal(t, 3868, response.Usage.PromptTokens)
	require.Equal(t, 3868, response.Usage.TotalTokens)
}

func TestGeminiStreamCountsOnlyCompletedPricedFunctionCalls(t *testing.T) {
	operation_setting.SetToolPriceForTest("priced_lookup", 5)
	t.Cleanup(func() {
		operation_setting.DeleteToolPriceForTest("priced_lookup")
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:streamGenerateContent", nil)
	willContinue := true
	completed := false
	chunks := []dto.GeminiChatResponse{
		{
			Candidates: []dto.GeminiChatCandidate{{Content: dto.GeminiChatContent{Parts: []dto.GeminiPart{{
				FunctionCall: &dto.FunctionCall{FunctionName: "priced_lookup", WillContinue: &willContinue},
			}}}}},
		},
		{
			Candidates: []dto.GeminiChatCandidate{{Content: dto.GeminiChatContent{Parts: []dto.GeminiPart{{
				FunctionCall: &dto.FunctionCall{FunctionName: "priced_lookup", WillContinue: &completed},
			}}}}},
		},
	}
	var streamBody strings.Builder
	for _, chunk := range chunks {
		data, err := common.Marshal(chunk)
		require.NoError(t, err)
		streamBody.WriteString("data: ")
		streamBody.Write(data)
		streamBody.WriteString("\n\n")
	}
	streamBody.WriteString("data: [DONE]\n\n")

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-2.5-flash",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gemini-2.5-flash"},
	}
	_, newAPIError := geminiStreamHandler(c, info, &http.Response{
		Body: io.NopCloser(strings.NewReader(streamBody.String())),
	}, func(_ string, _ *dto.GeminiChatResponse) bool { return true })

	require.Nil(t, newAPIError)
	require.NotNil(t, info.ResponsesUsageInfo)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, "priced_lookup")
	require.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools["priced_lookup"].CallCount)
}

func TestGeminiStreamHandlerMergesUsageMetadataAcrossFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	frames := []dto.GeminiChatResponse{
		{UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:        10,
			CachedContentTokenCount: 4,
		}},
		{UsageMetadata: dto.GeminiUsageMetadata{
			CandidatesTokenCount: 3,
			ThoughtsTokenCount:   2,
			TotalTokenCount:      15,
		}},
	}
	var streamBody strings.Builder
	for _, frame := range frames {
		data, err := common.Marshal(frame)
		require.NoError(t, err)
		streamBody.WriteString("data: ")
		streamBody.Write(data)
		streamBody.WriteString("\n\n")
	}
	streamBody.WriteString("data: [DONE]\n\n")
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gemini-test"}}

	usage, newAPIError := geminiStreamHandler(c, info, &http.Response{Body: io.NopCloser(strings.NewReader(streamBody.String()))}, func(_ string, _ *dto.GeminiChatResponse) bool {
		return true
	})

	require.Nil(t, newAPIError)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 15, usage.TotalTokens)
	require.Equal(t, 4, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 2, usage.CompletionTokenDetails.ReasoningTokens)
}

func TestBuildUsageFromGeminiMetadataMapsCandidateModalities(t *testing.T) {
	usage := buildUsageFromGeminiMetadata(dto.GeminiUsageMetadata{
		PromptTokenCount:     10,
		CandidatesTokenCount: 15,
		TotalTokenCount:      25,
		PromptTokensDetails: []dto.GeminiPromptTokensDetails{
			{Modality: " image ", TokenCount: 2},
			{Modality: "audio", TokenCount: 3},
			{Modality: "TEXT", TokenCount: 5},
		},
		CandidatesTokensDetails: []dto.GeminiPromptTokensDetails{
			{Modality: "TEXT", TokenCount: 5},
			{Modality: " image ", TokenCount: 4},
			{Modality: "IMAGE", TokenCount: 3},
			{Modality: "audio", TokenCount: 3},
		},
	}, 0)

	require.Equal(t, 2, usage.PromptTokensDetails.ImageTokens)
	require.Equal(t, 3, usage.PromptTokensDetails.AudioTokens)
	require.Equal(t, 5, usage.PromptTokensDetails.TextTokens)
	require.Equal(t, 5, usage.CompletionTokenDetails.TextTokens)
	require.Equal(t, 7, usage.CompletionTokenDetails.ImageTokens)
	require.Equal(t, 3, usage.CompletionTokenDetails.AudioTokens)
}

func TestMergeGeminiUsageMetadataAccumulatesModalityDetailsAcrossFrames(t *testing.T) {
	current := &dto.GeminiUsageMetadata{
		PromptTokenCount:     10,
		CandidatesTokenCount: 5,
		ThoughtsTokenCount:   2,
		TotalTokenCount:      17,
		CandidatesTokensDetails: []dto.GeminiPromptTokensDetails{
			{Modality: " image ", TokenCount: 2},
		},
	}
	next := dto.GeminiUsageMetadata{
		CandidatesTokenCount: 10,
		TotalTokenCount:      20,
		CandidatesTokensDetails: []dto.GeminiPromptTokensDetails{
			{Modality: "IMAGE", TokenCount: 3},
		},
	}

	merged := mergeGeminiUsageMetadata(current, next)
	usage := buildUsageFromGeminiMetadata(*merged, 0)

	require.Equal(t, 10, merged.CandidatesTokenCount)
	require.Equal(t, 0, merged.ThoughtsTokenCount)
	require.Equal(t, 20, merged.TotalTokenCount)
	require.Equal(t, 5, usage.CompletionTokenDetails.ImageTokens)
}

func TestGeminiChatStreamHandlerTerminatesClaudeToolCallOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	finishReason := "STOP"
	chunk := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Content: dto.GeminiChatContent{
				Role: "model",
				Parts: []dto.GeminiPart{{
					FunctionCall: &dto.FunctionCall{
						FunctionName: "lookup_weather",
						Arguments:    map[string]any{"city": "Shanghai"},
					},
				}},
			},
			FinishReason: &finishReason,
		}},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
			TotalTokenCount:      15,
		},
	}
	chunkData, err := common.Marshal(chunk)
	require.NoError(t, err)
	resp := &http.Response{Body: io.NopCloser(bytes.NewReader([]byte("data: " + string(chunkData) + "\n\ndata: [DONE]\n\n")))}

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatClaude,
		OriginModelName: "gemini-3-flash-preview",
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash-preview",
		},
	}

	usage, newAPIError := GeminiChatStreamHandler(c, info, resp)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	body := recorder.Body.String()
	require.Equal(t, 1, strings.Count(body, `"type":"message_stop"`), body)
	require.Contains(t, body, `"stop_reason":"tool_use"`)
}

func TestGeminiChatHandlerCompletionTokensExcludeToolUsePromptTokens(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatGemini,
		OriginModelName: "gemini-3-flash-preview",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash-preview",
		},
	}

	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "ok"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:        151,
			ToolUsePromptTokenCount: 18329,
			CandidatesTokenCount:    1089,
			ThoughtsTokenCount:      1120,
			TotalTokenCount:         20689,
		},
	}

	body, err := common.Marshal(payload)
	require.NoError(t, err)

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	usage, newAPIError := GeminiChatHandler(c, info, resp)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 18480, usage.PromptTokens)
	require.Equal(t, 2209, usage.CompletionTokens)
	require.Equal(t, 20689, usage.TotalTokens)
	require.Equal(t, 1120, usage.CompletionTokenDetails.ReasoningTokens)
}

func TestGeminiStreamHandlerCompletionTokensExcludeToolUsePromptTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-3-flash-preview",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash-preview",
		},
	}

	chunk := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "partial"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:        151,
			ToolUsePromptTokenCount: 18329,
			CandidatesTokenCount:    1089,
			ThoughtsTokenCount:      1120,
			TotalTokenCount:         20689,
		},
	}

	chunkData, err := common.Marshal(chunk)
	require.NoError(t, err)

	streamBody := []byte("data: " + string(chunkData) + "\n" + "data: [DONE]\n")
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(streamBody)),
	}

	usage, newAPIError := geminiStreamHandler(c, info, resp, func(_ string, _ *dto.GeminiChatResponse) bool {
		return true
	})
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 18480, usage.PromptTokens)
	require.Equal(t, 2209, usage.CompletionTokens)
	require.Equal(t, 20689, usage.TotalTokens)
	require.Equal(t, 1120, usage.CompletionTokenDetails.ReasoningTokens)
}

func TestGeminiTextGenerationHandlerPromptTokensIncludeToolUsePromptTokens(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-3-flash-preview:generateContent", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-3-flash-preview",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash-preview",
		},
	}

	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "ok"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:        151,
			ToolUsePromptTokenCount: 18329,
			CandidatesTokenCount:    1089,
			ThoughtsTokenCount:      1120,
			TotalTokenCount:         20689,
		},
	}

	body, err := common.Marshal(payload)
	require.NoError(t, err)

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	usage, newAPIError := GeminiTextGenerationHandler(c, info, resp)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 18480, usage.PromptTokens)
	require.Equal(t, 2209, usage.CompletionTokens)
	require.Equal(t, 20689, usage.TotalTokens)
	require.Equal(t, 1120, usage.CompletionTokenDetails.ReasoningTokens)
}

func TestGeminiChatHandlerUsesEstimatedPromptTokensWhenUsagePromptMissing(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatGemini,
		OriginModelName: "gemini-3-flash-preview",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash-preview",
		},
	}
	info.SetEstimatePromptTokens(20)

	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "ok"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:        0,
			ToolUsePromptTokenCount: 0,
			CandidatesTokenCount:    90,
			ThoughtsTokenCount:      10,
			TotalTokenCount:         110,
		},
	}

	body, err := common.Marshal(payload)
	require.NoError(t, err)

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	usage, newAPIError := GeminiChatHandler(c, info, resp)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 20, usage.PromptTokens)
	require.Equal(t, 100, usage.CompletionTokens)
	require.Equal(t, 110, usage.TotalTokens)
}

func TestGeminiStreamHandlerUsesEstimatedPromptTokensWhenUsagePromptMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-3-flash-preview",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash-preview",
		},
	}
	info.SetEstimatePromptTokens(20)

	chunk := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "partial"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:        0,
			ToolUsePromptTokenCount: 0,
			CandidatesTokenCount:    90,
			ThoughtsTokenCount:      10,
			TotalTokenCount:         110,
		},
	}

	chunkData, err := common.Marshal(chunk)
	require.NoError(t, err)

	streamBody := []byte("data: " + string(chunkData) + "\n" + "data: [DONE]\n")
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(streamBody)),
	}

	usage, newAPIError := geminiStreamHandler(c, info, resp, func(_ string, _ *dto.GeminiChatResponse) bool {
		return true
	})
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 20, usage.PromptTokens)
	require.Equal(t, 100, usage.CompletionTokens)
	require.Equal(t, 110, usage.TotalTokens)
}

func TestGeminiTextGenerationHandlerUsesEstimatedPromptTokensWhenUsagePromptMissing(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-3-flash-preview:generateContent", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-3-flash-preview",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash-preview",
		},
	}
	info.SetEstimatePromptTokens(20)

	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "ok"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:        0,
			ToolUsePromptTokenCount: 0,
			CandidatesTokenCount:    90,
			ThoughtsTokenCount:      10,
			TotalTokenCount:         110,
		},
	}

	body, err := common.Marshal(payload)
	require.NoError(t, err)

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	usage, newAPIError := GeminiTextGenerationHandler(c, info, resp)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 20, usage.PromptTokens)
	require.Equal(t, 100, usage.CompletionTokens)
	require.Equal(t, 110, usage.TotalTokens)
}
