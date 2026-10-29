package openai

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesReasoningEffortPayloadAndUsageLog(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		wantModel  string
		wantEffort string
		wantAuto   bool
	}{
		{"none", `{"model":"gpt-6-astra","reasoning":{"effort":"none","summary":"auto"}}`, "gpt-6-astra", "high", true},
		{"stream none", `{"model":"gpt-6-astra","stream":true,"reasoning":{"effort":"none"}}`, "gpt-6-astra", "high", true},
		{"missing reasoning", `{"model":"gpt-6-astra"}`, "gpt-6-astra", "high", true},
		{"empty effort", `{"model":"gpt-6-astra","reasoning":{"summary":"detailed"}}`, "gpt-6-astra", "high", true},
		{"none model suffix", `{"model":"gpt-6-astra-none","reasoning":{"effort":"low"}}`, "gpt-6-astra", "high", true},
		{"low model suffix", `{"model":"gpt-6-astra-low","reasoning":{"effort":"none"}}`, "gpt-6-astra", "low", false},
		{"low", `{"model":"gpt-6-astra","reasoning":{"effort":"low"}}`, "gpt-6-astra", "low", false},
		{"medium", `{"model":"gpt-6-astra","reasoning":{"effort":"medium"}}`, "gpt-6-astra", "medium", false},
		{"high", `{"model":"gpt-6-astra","reasoning":{"effort":"high"}}`, "gpt-6-astra", "high", false},
		{"xhigh", `{"model":"gpt-6-astra","reasoning":{"effort":"xhigh"}}`, "gpt-6-astra", "xhigh", false},
		{"max", `{"model":"gpt-6-astra","reasoning":{"effort":"max"}}`, "gpt-6-astra", "max", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var request dto.OpenAIResponsesRequest
			require.NoError(t, common.UnmarshalJsonStr(tc.body, &request))
			wantSummary := ""
			if request.Reasoning != nil {
				wantSummary = request.Reasoning.Summary
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			info := &relaycommon.RelayInfo{
				ChannelMeta:         &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
				ReasoningEffortAuto: true, // Explicit efforts must clear a previous automatic selection.
			}
			converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(ctx, info, request)
			require.NoError(t, err)
			payload, err := common.Marshal(converted)
			require.NoError(t, err)
			var upstream dto.OpenAIResponsesRequest
			require.NoError(t, common.Unmarshal(payload, &upstream))
			require.Equal(t, tc.wantModel, upstream.Model)
			require.NotNil(t, upstream.Reasoning)
			require.Equal(t, tc.wantEffort, upstream.Reasoning.Effort)
			require.Equal(t, wantSummary, upstream.Reasoning.Summary)
			require.Equal(t, request.Stream, upstream.Stream)

			other := service.GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 1, 0, 1)
			require.Equal(t, tc.wantEffort, other["reasoning_effort"])
			if tc.wantAuto {
				require.Equal(t, true, other["reasoning_effort_auto"])
			} else {
				require.NotContains(t, other, "reasoning_effort_auto")
			}
		})
	}
}

func TestChatReasoningEffortPayloadAndUsageLog(t *testing.T) {
	for _, tc := range []struct {
		name        string
		model       string
		effort      string
		channelType int
		wantModel   string
		wantEffort  string
		wantAuto    bool
	}{
		{"missing GPT-6 effort", "gpt-6-astra", "", constant.ChannelTypeOpenAI, "gpt-6-astra", "high", true},
		{"none GPT-6 effort", "gpt-6-astra", "none", constant.ChannelTypeOpenAI, "gpt-6-astra", "high", true},
		{"explicit low", "gpt-6-astra", "low", constant.ChannelTypeOpenAI, "gpt-6-astra", "low", false},
		{"explicit medium", "gpt-6-astra", "medium", constant.ChannelTypeOpenAI, "gpt-6-astra", "medium", false},
		{"explicit high", "gpt-6-astra", "high", constant.ChannelTypeOpenAI, "gpt-6-astra", "high", false},
		{"explicit xhigh", "gpt-6-astra", "xhigh", constant.ChannelTypeOpenAI, "gpt-6-astra", "xhigh", false},
		{"explicit max", "gpt-6-astra", "max", constant.ChannelTypeOpenAI, "gpt-6-astra", "max", false},
		{"none suffix", "gpt-6-astra-none", "low", constant.ChannelTypeOpenAI, "gpt-6-astra", "high", true},
		{"low suffix", "gpt-6-astra-low", "none", constant.ChannelTypeOpenAI, "gpt-6-astra", "low", false},
		{"missing GPT-5 effort", "gpt-5", "", constant.ChannelTypeOpenAI, "gpt-5", "high", true},
		{"GPT-5 suffix", "gpt-5-low", "", constant.ChannelTypeOpenAI, "gpt-5", "low", false},
		{"missing o3 effort", "o3", "", constant.ChannelTypeOpenAI, "o3", "high", true},
		{"Azure", "gpt-6-astra", "", constant.ChannelTypeAzure, "gpt-6-astra", "high", true},
		{"non-reasoning model", "gpt-4o", "", constant.ChannelTypeOpenAI, "gpt-4o", "", false},
		{"legacy o1", "o1-mini", "", constant.ChannelTypeOpenAI, "o1-mini", "", false},
		{"chat model", "gpt-5-chat-latest", "", constant.ChannelTypeOpenAI, "gpt-5-chat-latest", "", false},
		{"other provider", "gpt-6-astra", "", constant.ChannelTypeOpenRouter, "gpt-6-astra", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       tc.channelType,
					UpstreamModelName: tc.model,
				},
			}
			request := &dto.GeneralOpenAIRequest{
				Model:           tc.model,
				ReasoningEffort: tc.effort,
				Messages:        []dto.Message{{Role: "user", Content: "hello"}},
			}
			converted, err := (&Adaptor{}).ConvertOpenAIRequest(ctx, info, request)
			require.NoError(t, err)
			payload, err := common.Marshal(converted)
			require.NoError(t, err)
			var upstream map[string]any
			require.NoError(t, common.Unmarshal(payload, &upstream))
			require.Equal(t, tc.wantModel, upstream["model"])
			other := service.GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 1, 0, 1)
			if tc.wantEffort == "" {
				require.NotContains(t, upstream, "reasoning_effort")
				require.NotContains(t, other, "reasoning_effort")
			} else {
				require.Equal(t, tc.wantEffort, upstream["reasoning_effort"])
				require.Equal(t, tc.wantEffort, other["reasoning_effort"])
			}
			if tc.wantAuto {
				require.Equal(t, true, other["reasoning_effort_auto"])
			} else {
				require.NotContains(t, other, "reasoning_effort_auto")
			}
		})
	}
}
