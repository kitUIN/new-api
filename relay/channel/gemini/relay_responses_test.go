package gemini

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiResponsesHandlerReturnsOpenAIResponsesJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-responses-test")

	info := newGeminiResponsesRelayInfo(false)
	payload := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "hello"},
					},
				},
				GroundingMetadata: &dto.GeminiGroundingMetadata{
					WebSearchQueries:  []string{"Gemini release notes"},
					GroundingChunks:   []byte(`[{"web":{"uri":"https://ai.google.dev/gemini-api/docs","title":"Gemini API"}}]`),
					GroundingSupports: []byte(`[{"segment":{"partIndex":0,"startIndex":0,"endIndex":5,"text":"hello"},"groundingChunkIndices":[0]}]`),
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     2,
			CandidatesTokenCount: 3,
			TotalTokenCount:      5,
		},
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)

	usage, newAPIError := GeminiResponsesHandler(c, info, &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	})
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 2, usage.PromptTokens)
	assert.Equal(t, 3, usage.CompletionTokens)

	got := recorder.Body.String()
	assert.Contains(t, got, `"object":"response"`)
	assert.Contains(t, got, `"status":"completed"`)
	assert.Contains(t, got, `"type":"output_text"`)
	assert.Contains(t, got, `"text":"hello"`)
	assert.Contains(t, got, `"type":"web_search_call"`)
	assert.Contains(t, got, `"queries":["Gemini release notes"]`)
	assert.Contains(t, got, `"type":"url_citation"`)
	assert.Contains(t, got, `"url":"https://ai.google.dev/gemini-api/docs"`)
	assert.Contains(t, got, `"start_index":0`)
	assert.Contains(t, got, `"end_index":5`)
	assert.Contains(t, got, `"input_tokens":2`)
	assert.Contains(t, got, `"output_tokens":3`)
	assert.NotContains(t, got, `"choices"`)
	assert.NotContains(t, got, `"candidates"`)
	assert.True(t, c.GetBool("gemini_google_search_call"))
}

func TestGeminiResponsesHandlerReturnsPromptBlockedForBlockedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reason := "SAFETY"
	body, err := common.Marshal(dto.GeminiChatResponse{
		PromptFeedback: &dto.GeminiChatPromptFeedback{BlockReason: &reason},
		UsageMetadata:  dto.GeminiUsageMetadata{PromptTokenCount: 3, TotalTokenCount: 3},
	})
	require.NoError(t, err)

	usage, newAPIError := GeminiResponsesHandler(c, newGeminiResponsesRelayInfo(false), &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	})

	require.NotNil(t, usage)
	assert.Equal(t, 3, usage.PromptTokens)
	require.NotNil(t, newAPIError)
	assert.Equal(t, types.ErrorCodePromptBlocked, newAPIError.GetErrorCode())
	assert.Equal(t, http.StatusBadRequest, newAPIError.StatusCode)
	assert.Contains(t, common.GetContextKeyString(c, constant.ContextKeyAdminRejectReason), "SAFETY")
	assert.Empty(t, recorder.Body.String())
}

func TestGeminiResponsesHandlerClosesBodyOnReadError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-responses-read-error-test")

	body := &failingReadCloser{}
	usage, newAPIError := GeminiResponsesHandler(c, newGeminiResponsesRelayInfo(false), &http.Response{Body: body})

	require.Nil(t, usage)
	require.NotNil(t, newAPIError)
	assert.True(t, body.closed)
}

func TestGeminiResponsesStreamHandlerReturnsOpenAIResponsesSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-responses-stream-test")

	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	info := newGeminiResponsesRelayInfo(true)
	first := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "hello"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     2,
			CandidatesTokenCount: 3,
			TotalTokenCount:      5,
		},
	}
	stop := "STOP"
	final := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				FinishReason: &stop,
				Content: dto.GeminiChatContent{
					Role:  "model",
					Parts: []dto.GeminiPart{{Text: ""}},
				},
				GroundingMetadata: &dto.GeminiGroundingMetadata{
					WebSearchQueries:  []string{"Gemini streaming"},
					GroundingChunks:   []byte(`[{"web":{"uri":"https://example.com/stream","title":"Stream source"}}]`),
					GroundingSupports: []byte(`[{"segment":{"partIndex":0,"startIndex":0,"endIndex":5,"text":"hello"},"groundingChunkIndices":[0]}]`),
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     2,
			CandidatesTokenCount: 3,
			TotalTokenCount:      5,
		},
	}
	firstData, err := common.Marshal(first)
	require.NoError(t, err)
	finalData, err := common.Marshal(final)
	require.NoError(t, err)
	streamBody := strings.Join([]string{
		"data: " + string(firstData),
		"",
		"data: " + string(finalData),
		"",
		"data: [DONE]",
		"",
	}, "\n")

	usage, newAPIError := GeminiResponsesStreamHandler(c, info, &http.Response{
		Body: io.NopCloser(strings.NewReader(streamBody)),
	})
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 5, usage.TotalTokens)

	got := recorder.Body.String()
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Contains(t, got, `event: response.created`)
	assert.Contains(t, got, `event: response.output_text.delta`)
	assert.Contains(t, got, `"delta":"hello"`)
	assert.Contains(t, got, `event: response.completed`)
	assert.Contains(t, got, `event: response.web_search_call.searching`)
	assert.Contains(t, got, `event: response.output_text.annotation.added`)
	assert.Contains(t, got, `"url":"https://example.com/stream"`)
	assert.Contains(t, got, `"annotations":[{"end_index":5`)
	assert.Contains(t, got, `"queries":["Gemini streaming"]`)
	assert.Contains(t, got, `"input_tokens":2`)
	assert.Contains(t, got, `"output_tokens":3`)
	assert.NotContains(t, got, `"choices"`)
	assert.NotContains(t, got, `"candidates"`)
	requireOrderedGeminiResponsesSubstrings(t, got,
		`event: response.created`,
		`event: response.output_item.added`,
		`event: response.output_text.delta`,
		`event: response.output_text.annotation.added`,
		`event: response.output_text.done`,
		`event: response.web_search_call.in_progress`,
		`event: response.web_search_call.searching`,
		`event: response.web_search_call.completed`,
		`event: response.completed`,
	)
}

func TestGeminiResponsesStreamHandlerPreservesReasoningAndFunctionCalls(t *testing.T) {
	stop := "STOP"
	frames := []dto.GeminiChatResponse{
		{
			Candidates: []dto.GeminiChatCandidate{{
				Content: dto.GeminiChatContent{
					Role:  "model",
					Parts: []dto.GeminiPart{{Text: "checking", Thought: true}},
				},
			}},
		},
		{
			Candidates: []dto.GeminiChatCandidate{{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{{FunctionCall: &dto.FunctionCall{
						FunctionName: "lookup",
						Arguments:    map[string]any{"q": "weather"},
					}}},
				},
			}},
		},
		{
			Candidates: []dto.GeminiChatCandidate{{
				FinishReason: &stop,
				Content:      dto.GeminiChatContent{Role: "model"},
			}},
			UsageMetadata: dto.GeminiUsageMetadata{
				PromptTokenCount:     4,
				CandidatesTokenCount: 3,
				ThoughtsTokenCount:   2,
				TotalTokenCount:      9,
			},
		},
	}

	recorder, usage, newAPIError := runGeminiResponsesStream(t, frames, true)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 2, usage.CompletionTokenDetails.ReasoningTokens)

	got := recorder.Body.String()
	assert.Contains(t, got, `event: response.reasoning_summary_text.delta`)
	assert.Contains(t, got, `"delta":"checking"`)
	assert.Contains(t, got, `event: response.function_call_arguments.delta`)
	assert.Contains(t, got, `"name":"lookup"`)
	assert.Contains(t, got, `"arguments":"{\"q\":\"weather\"}"`)
	assert.Contains(t, got, `event: response.completed`)
}

func TestGeminiResponsesStreamHandlerReconstructsPartialFunctionCall(t *testing.T) {
	willContinue := true
	completed := false
	finishReason := "STOP"
	firstValue := "San"
	secondValue := " Francisco"
	frames := []dto.GeminiChatResponse{
		{
			Candidates: []dto.GeminiChatCandidate{
				{
					Content: dto.GeminiChatContent{
						Role: "model",
						Parts: []dto.GeminiPart{
							{FunctionCall: &dto.FunctionCall{
								ID:           "call_weather",
								FunctionName: "lookup_weather",
								PartialArgs: []dto.GeminiPartialArg{
									{JSONPath: "$.city", StringValue: &firstValue},
								},
								WillContinue: &willContinue,
							}},
						},
					},
				},
			},
		},
		{
			Candidates: []dto.GeminiChatCandidate{
				{
					FinishReason: &finishReason,
					Content: dto.GeminiChatContent{
						Role: "model",
						Parts: []dto.GeminiPart{
							{FunctionCall: &dto.FunctionCall{
								ID:           "call_weather",
								FunctionName: "lookup_weather",
								PartialArgs: []dto.GeminiPartialArg{
									{JSONPath: "$.city", StringValue: &secondValue},
								},
								WillContinue: &completed,
							}},
						},
					},
				},
			},
		},
	}

	recorder, _, newAPIError := runGeminiResponsesStream(t, frames, true)
	require.Nil(t, newAPIError)
	got := recorder.Body.String()
	assert.Contains(t, got, `"id":"call_weather"`)
	assert.Contains(t, got, `"name":"lookup_weather"`)
	assert.Contains(t, got, `"arguments":"{\"city\":\"San Francisco\"}"`)
	assert.NotContains(t, got, `event: response.failed`)
}

func TestGeminiResponsesStreamHandlerFailsIncompletePartialFunctionCall(t *testing.T) {
	willContinue := true
	value := "partial"
	frames := []dto.GeminiChatResponse{{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{FunctionCall: &dto.FunctionCall{
							ID:           "call_incomplete",
							FunctionName: "lookup",
							PartialArgs:  []dto.GeminiPartialArg{{JSONPath: "$.q", StringValue: &value}},
							WillContinue: &willContinue,
						}},
					},
				},
			},
		},
	}}

	recorder, _, newAPIError := runGeminiResponsesStream(t, frames, true)
	require.Nil(t, newAPIError)
	got := recorder.Body.String()
	assert.Contains(t, got, `event: response.failed`)
	assert.Contains(t, got, `incomplete function call`)
	assert.NotContains(t, got, `event: response.completed`)
}

func TestGeminiResponsesStreamHandlerReturnsFailedForBlockedPrompt(t *testing.T) {
	reason := "SAFETY"
	frames := []dto.GeminiChatResponse{{
		PromptFeedback: &dto.GeminiChatPromptFeedback{BlockReason: &reason},
	}}

	recorder, _, newAPIError := runGeminiResponsesStream(t, frames, true)
	require.Nil(t, newAPIError)
	got := recorder.Body.String()
	assert.Contains(t, got, `event: error`)
	assert.Contains(t, got, `"code":"prompt_blocked"`)
	assert.Contains(t, got, `event: response.failed`)
	assert.Contains(t, got, `"status":"failed"`)
	assert.NotContains(t, got, `event: response.completed`)
}

func TestGeminiResponsesStreamHandlerReturnsFailedForPrematureEOF(t *testing.T) {
	frames := []dto.GeminiChatResponse{{
		Candidates: []dto.GeminiChatCandidate{{
			Content: dto.GeminiChatContent{
				Role:  "model",
				Parts: []dto.GeminiPart{{Text: "partial"}},
			},
		}},
	}}

	recorder, _, newAPIError := runGeminiResponsesStream(t, frames, false)
	require.Nil(t, newAPIError)
	got := recorder.Body.String()
	assert.Contains(t, got, `"delta":"partial"`)
	assert.Contains(t, got, `event: response.failed`)
	assert.Contains(t, got, `Gemini stream ended before a terminal candidate`)
	assert.NotContains(t, got, `event: response.completed`)
}

func TestGeminiResponsesStreamHandlerMapsMaxTokensToIncomplete(t *testing.T) {
	finishReason := "MAX_TOKENS"
	frames := []dto.GeminiChatResponse{{
		Candidates: []dto.GeminiChatCandidate{{
			FinishReason: &finishReason,
			Content: dto.GeminiChatContent{
				Role:  "model",
				Parts: []dto.GeminiPart{{Text: "truncated"}},
			},
		}},
	}}

	recorder, _, newAPIError := runGeminiResponsesStream(t, frames, false)
	require.Nil(t, newAPIError)
	got := recorder.Body.String()
	assert.Contains(t, got, `event: response.incomplete`)
	assert.Contains(t, got, `"incomplete_details":{"reason":"max_output_tokens"}`)
	assert.NotContains(t, got, `event: response.failed`)
}

func runGeminiResponsesStream(t *testing.T, frames []dto.GeminiChatResponse, includeDone bool) (*httptest.ResponseRecorder, *dto.Usage, *types.NewAPIError) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "gemini-responses-extra-stream-test")

	var streamBody strings.Builder
	for _, frame := range frames {
		data, err := common.Marshal(frame)
		require.NoError(t, err)
		streamBody.WriteString("data: ")
		streamBody.Write(data)
		streamBody.WriteString("\n\n")
	}
	if includeDone {
		streamBody.WriteString("data: [DONE]\n\n")
	}

	usage, newAPIError := GeminiResponsesStreamHandler(c, newGeminiResponsesRelayInfo(true), &http.Response{
		Body: io.NopCloser(strings.NewReader(streamBody.String())),
	})
	return recorder, usage, newAPIError
}

func newGeminiResponsesRelayInfo(isStream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		IsStream:        isStream,
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RequestURLPath:  "/v1/responses",
		DisablePing:     true,
		OriginModelName: "gemini-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-test",
		},
	}
}

type failingReadCloser struct {
	closed bool
}

func (r *failingReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func (r *failingReadCloser) Close() error {
	r.closed = true
	return nil
}

func requireOrderedGeminiResponsesSubstrings(t *testing.T, s string, parts ...string) {
	t.Helper()
	offset := 0
	for _, part := range parts {
		idx := strings.Index(s[offset:], part)
		require.NotEqualf(t, -1, idx, "missing %q after byte offset %d", part, offset)
		offset += idx + len(part)
	}
}
