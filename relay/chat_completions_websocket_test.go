package relay

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestUpstreamWebSocketForcesChatConversionDespitePassthrough(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	settings.PassThroughRequestEnabled = true
	t.Cleanup(func() { settings.PassThroughRequestEnabled = previous })
	for _, channelType := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeAzure, constant.ChannelTypeCodex} {
		t.Run(fmt.Sprint(channelType), func(t *testing.T) {
			c, _ := newResponsesWebSocketTestContext(t)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
			common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{UseResponsesWebSocket: true})
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
			request := &dto.GeneralOpenAIRequest{Model: "gpt-4o", N: common.GetPointer(2)}
			info := relaycommon.GenRelayInfoOpenAI(c, request)
			apiErr := TextHelper(c, info)
			require.NotNil(t, apiErr)
			// This request is rejected by the Responses converter before dialing.
			require.Contains(t, apiErr.Error(), "n>1 is not supported in responses compatibility mode")
			require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		})
	}
}

func TestChatCompletionsViaResponsesWebSocket(t *testing.T) {
	for _, channelType := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeAzure, constant.ChannelTypeCodex} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/stream=%t", channelType, stream), func(t *testing.T) {
				upgrader := websocket.Upgrader{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.NotEqual(t, "/v1/chat/completions", r.URL.Path)
					conn, err := upgrader.Upgrade(w, r, nil)
					if !assert.NoError(t, err) {
						return
					}
					defer conn.Close()
					_, payload, err := conn.ReadMessage()
					if !assert.NoError(t, err) {
						return
					}
					assert.Equal(t, "response.create", gjson.GetBytes(payload, "type").String())
					assert.Equal(t, "gpt-test", gjson.GetBytes(payload, "model").String())
					assert.Equal(t, "hello", gjson.GetBytes(payload, "input.0.content").String())
					assert.Equal(t, "false", gjson.GetBytes(payload, "parallel_tool_calls").Raw)
					assert.False(t, gjson.GetBytes(payload, "messages").Exists())
					assert.False(t, gjson.GetBytes(payload, "stream").Exists())
					if channelType != constant.ChannelTypeCodex {
						assert.Equal(t, "0", gjson.GetBytes(payload, "temperature").Raw)
						assert.Equal(t, "0", gjson.GetBytes(payload, "max_output_tokens").Raw)
					}
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created","response":{"id":"resp_chat","model":"gpt-test"}}`))
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.output_text.delta","delta":"world"}`))
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed","response":{"id":"resp_chat","object":"response","model":"gpt-test","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"world"}]}],"usage":{"input_tokens":9,"output_tokens":2,"total_tokens":11,"input_tokens_details":{"cached_tokens":3}}}}`))
				}))
				defer server.Close()
				gin.SetMode(gin.TestMode)
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
				ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
				defer cancel()
				c.Request = c.Request.WithContext(ctx)
				var request dto.GeneralOpenAIRequest
				require.NoError(t, common.UnmarshalJsonStr(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"temperature":0,"max_tokens":0,"parallel_tool_calls":false}`, &request))
				request.Stream = &stream
				info := &relaycommon.RelayInfo{
					ChannelMeta: &relaycommon.ChannelMeta{ChannelType: channelType, ChannelBaseUrl: server.URL, ApiKey: "test-key", UpstreamModelName: "gpt-test", ChannelOtherSettings: dto.ChannelOtherSettings{UseResponsesWebSocket: true}},
					RelayMode:   relayconstant.RelayModeChatCompletions, RequestURLPath: "/v1/chat/completions",
					IsStream: stream, ShouldIncludeUsage: true, RelayFormat: types.RelayFormatOpenAI, Transport: types.RelayTransportHTTP,
				}
				var adaptor channel.Adaptor = &openai.Adaptor{}
				if channelType == constant.ChannelTypeCodex {
					adaptor = &codex.Adaptor{}
					info.ApiKey = `{"access_token":"test-token","account_id":"test-account"}`
				}
				adaptor.Init(info)
				usage, apiErr := chatCompletionsViaResponses(c, info, adaptor, &request)
				require.Nil(t, apiErr)
				require.Equal(t, 11, usage.TotalTokens)
				require.Equal(t, 3, usage.PromptTokensDetails.CachedTokens)
				require.Equal(t, relayconstant.RelayModeChatCompletions, info.RelayMode)
				require.Equal(t, "/v1/chat/completions", info.RequestURLPath)
				if stream {
					require.Contains(t, recorder.Body.String(), "chat.completion.chunk")
					require.Contains(t, recorder.Body.String(), `"content":"world"`)
					require.Contains(t, recorder.Body.String(), "data: [DONE]")
					require.NotContains(t, recorder.Body.String(), "response.completed")
				} else {
					require.Equal(t, "chat.completion", gjson.Get(recorder.Body.String(), "object").String())
					require.Equal(t, "world", gjson.Get(recorder.Body.String(), "choices.0.message.content").String())
				}
			})
		}
	}
}
