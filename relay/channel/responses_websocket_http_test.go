package channel_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const completedResponse = `{"type":"response.completed","response":{"id":"resp_test","object":"response","model":"gpt-test","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":9,"output_tokens":2,"total_tokens":11,"input_tokens_details":{"cached_tokens":3}},"custom_field":9007199254740993}}`

func bridgeTestInfo(t *testing.T, baseURL string, channelType int, stream bool) (*gin.Context, *relaycommon.RelayInfo, channel.Adaptor) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	t.Cleanup(cancel)
	c.Request = c.Request.WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: channelType, ChannelBaseUrl: baseURL, ApiKey: "upstream-key",
			ChannelOtherSettings: dto.ChannelOtherSettings{UseResponsesWebSocket: true},
			HeadersOverride:      map[string]any{"X-Upstream-Test": "custom"},
			UpstreamModelName:    "gpt-test",
		},
		RelayMode: relayconstant.RelayModeResponses, RequestURLPath: "/v1/responses", IsStream: stream,
		RelayFormat: types.RelayFormatOpenAIResponses, Transport: types.RelayTransportHTTP,
	}
	var adaptor channel.Adaptor = &openai.Adaptor{}
	if channelType == constant.ChannelTypeCodex {
		adaptor = &codex.Adaptor{}
		info.ApiKey = `{"access_token":"codex-token","account_id":"account-test"}`
	}
	adaptor.Init(info)
	return c, info, adaptor
}

func TestResponsesWebSocketHTTPBridge(t *testing.T) {
	for _, channelType := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeAzure, constant.ChannelTypeCodex} {
		for _, stream := range []bool{false, true} {
			t.Run(common.Interface2String(channelType)+"/stream="+common.Interface2String(stream), func(t *testing.T) {
				upgrader := websocket.Upgrader{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "custom", r.Header.Get("X-Upstream-Test"))
					switch channelType {
					case constant.ChannelTypeAzure:
						assert.Equal(t, "/openai/v1/responses", r.URL.Path)
						assert.Equal(t, "preview", r.URL.Query().Get("api-version"))
						assert.Equal(t, "upstream-key", r.Header.Get("api-key"))
					case constant.ChannelTypeCodex:
						assert.Equal(t, "/backend-api/codex/responses", r.URL.Path)
						assert.Equal(t, "Bearer codex-token", r.Header.Get("Authorization"))
						assert.Equal(t, "account-test", r.Header.Get("chatgpt-account-id"))
					default:
						assert.Equal(t, "/v1/responses", r.URL.Path)
						assert.Equal(t, "Bearer upstream-key", r.Header.Get("Authorization"))
					}
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
					assert.False(t, gjson.GetBytes(payload, "stream").Exists())
					assert.False(t, gjson.GetBytes(payload, "background").Exists())
					assert.False(t, gjson.GetBytes(payload, "stream_options").Exists())
					assert.Equal(t, "0", gjson.GetBytes(payload, "temperature").Raw)
					assert.Equal(t, "false", gjson.GetBytes(payload, "store").Raw)
					assert.Equal(t, "false", gjson.GetBytes(payload, "parallel_tool_calls").Raw)
					assert.Equal(t, "9007199254740993", gjson.GetBytes(payload, "custom.id").Raw)
					assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("{\n\"type\":\"response.created\",\n\"response\":{\"id\":\"resp_test\"}\n}")))
					assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.output_text.delta","delta":"hello"}`)))
					assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(completedResponse)))
					// The bridge must terminate on the terminal event, not wait for EOF.
					_, _, _ = conn.ReadMessage()
				}))
				defer server.Close()
				c, info, adaptor := bridgeTestInfo(t, server.URL, channelType, stream)
				upstream, err := adaptor.DoRequest(c, info, strings.NewReader(`{"model":"gpt-test","input":"hi","stream":true,"stream_options":{},"background":false,"temperature":0,"store":false,"parallel_tool_calls":false,"custom":{"id":9007199254740993}}`))
				require.NoError(t, err)
				resp := upstream.(*http.Response)
				require.Equal(t, http.StatusOK, resp.StatusCode)
				usage, apiErr := adaptor.DoResponse(c, resp, info)
				require.Nil(t, apiErr)
				require.Equal(t, 11, usage.(*dto.Usage).TotalTokens)
				require.Equal(t, 3, usage.(*dto.Usage).PromptTokensDetails.CachedTokens)
				require.Equal(t, types.RelayTransportHTTP, info.Transport)
				require.Equal(t, types.RelayTransportWebSocket, info.UpstreamTransport)
				require.Equal(t, server.URL, info.ChannelBaseUrl)
			})
		}
	}
}

func TestResponsesWebSocketHTTPBridgeLeavesOtherRequestsOnHTTP(t *testing.T) {
	if service.GetHttpClient() == nil {
		service.InitHttpClient()
	}
	for _, scenario := range []string{"disabled", "compact", "unsupported-channel"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.False(t, websocket.IsWebSocketUpgrade(r))
				_, _ = w.Write([]byte(`{"id":"http-response"}`))
			}))
			defer server.Close()
			c, info, adaptor := bridgeTestInfo(t, server.URL, constant.ChannelTypeOpenAI, false)
			switch scenario {
			case "disabled":
				info.ChannelOtherSettings.UseResponsesWebSocket = false
			case "compact":
				info.RelayMode = relayconstant.RelayModeResponsesCompact
				info.RequestURLPath += "/compact"
			case "unsupported-channel":
				info.ChannelType = constant.ChannelTypeOpenRouter
			}
			upstream, err := adaptor.DoRequest(c, info, strings.NewReader(`{"model":"gpt-test"}`))
			require.NoError(t, err)
			resp := upstream.(*http.Response)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestResponsesWebSocketHTTPBridgeErrors(t *testing.T) {
	for _, scenario := range []string{"handshake", "first-error", "later-error", "failed", "disconnect", "binary", "invalid-json", "missing-response", "incomplete"} {
		t.Run(scenario, func(t *testing.T) {
			upgrader := websocket.Upgrader{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "handshake" {
					w.Header().Set("Retry-After", "3")
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"busy"}}`))
					return
				}
				conn, err := upgrader.Upgrade(w, r, nil)
				if !assert.NoError(t, err) {
					return
				}
				defer conn.Close()
				if _, _, err = conn.ReadMessage(); !assert.NoError(t, err) {
					return
				}
				if scenario != "first-error" {
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created"}`))
				}
				switch scenario {
				case "first-error", "later-error":
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","status":429,"error":{"code":"rate_limit_exceeded","message":"busy"}}`))
				case "failed":
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.failed","response":{"error":{"code":"server_error","message":"failed"}}}`))
				case "binary":
					_ = conn.WriteMessage(websocket.BinaryMessage, []byte(`{}`))
				case "invalid-json":
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`not-json`))
				case "missing-response":
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed"}`))
				case "incomplete":
					_ = conn.WriteMessage(websocket.TextMessage, []byte(strings.ReplaceAll(completedResponse, "completed", "incomplete")))
				}
			}))
			defer server.Close()
			c, info, adaptor := bridgeTestInfo(t, server.URL, constant.ChannelTypeOpenAI, true)
			upstream, err := adaptor.DoRequest(c, info, strings.NewReader(`{"model":"gpt-test"}`))
			require.NoError(t, err)
			resp := upstream.(*http.Response)
			defer resp.Body.Close()
			if scenario == "handshake" || scenario == "first-error" {
				require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
				if scenario == "handshake" {
					require.Equal(t, "3", resp.Header.Get("Retry-After"))
				}
				return
			}
			usage, apiErr := adaptor.DoResponse(c, resp, info)
			if scenario == "incomplete" {
				require.Nil(t, apiErr)
				require.Equal(t, 11, usage.(*dto.Usage).TotalTokens)
			} else {
				require.NotNil(t, apiErr)
				require.True(t, types.IsSkipRetryError(apiErr), "must not replay a partially sent stream")
			}
		})
	}
}

func TestResponsesWebSocketHTTPBridgeCancellation(t *testing.T) {
	for _, action := range []string{"cancel-before-first", "cancel-body", "close-body"} {
		t.Run(action, func(t *testing.T) {
			ready := make(chan struct{})
			disconnected := make(chan struct{})
			upgrader := websocket.Upgrader{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if !assert.NoError(t, err) {
					return
				}
				defer conn.Close()
				defer close(disconnected)
				if _, _, err = conn.ReadMessage(); !assert.NoError(t, err) {
					return
				}
				if action != "cancel-before-first" {
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created"}`))
				}
				close(ready)
				_, _, _ = conn.ReadMessage()
			}))
			defer server.Close()
			c, info, adaptor := bridgeTestInfo(t, server.URL, constant.ChannelTypeOpenAI, true)
			ctx, cancel := context.WithCancel(c.Request.Context())
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			done := make(chan error, 1)
			if action == "cancel-before-first" {
				go func() { _, err := adaptor.DoRequest(c, info, strings.NewReader(`{}`)); done <- err }()
				<-ready
				cancel()
			} else {
				upstream, err := adaptor.DoRequest(c, info, strings.NewReader(`{}`))
				require.NoError(t, err)
				body := upstream.(*http.Response).Body
				defer body.Close()
				go func() { _, err := io.ReadAll(body); done <- err }()
				if action == "close-body" {
					_ = body.Close()
				} else {
					cancel()
				}
			}
			select {
			case err := <-done:
				require.Error(t, err)
			case <-time.After(2 * time.Second):
				t.Fatal("read was not interrupted")
			}
			select {
			case <-disconnected:
			case <-time.After(2 * time.Second):
				t.Fatal("upstream connection leaked")
			}
		})
	}
}

func TestResponsesWebSocketHTTPBridgeRejectsBackground(t *testing.T) {
	c, info, adaptor := bridgeTestInfo(t, "http://unused.invalid", constant.ChannelTypeOpenAI, true)
	upstream, err := adaptor.DoRequest(c, info, strings.NewReader(`{"background":true}`))
	require.NoError(t, err)
	resp := upstream.(*http.Response)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
