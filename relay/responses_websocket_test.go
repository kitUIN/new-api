package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func newResponsesWebSocketTestContext(t *testing.T) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelName, "test")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://api.example.com")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "upstream-key")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{})
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]interface{}{})
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, map[string]interface{}{})
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{AcceptUnsetRatioModel: true})
	request := &dto.OpenAIResponsesRequest{Model: "gpt-4o"}
	info := relaycommon.GenRelayInfoResponsesWebSocket(c, request, nil)
	info.RequestId = "request-test"
	return c, info
}

func TestPrepareResponsesWebSocketRoundPreservesEnvelopeAndExplicitZero(t *testing.T) {
	c, info := newResponsesWebSocketTestContext(t)
	raw := []byte(`{
  "type":"response.create",
  "model":"gpt-4o",
  "input":[],
  "generate":false,
  "stream":true,
  "stream_id":"stream_1",
  "previous_response_id":"resp_1",
  "max_output_tokens":0,
  "temperature":0,
  "client_event_id":"evt_1"
}`)
	event, err := dto.ParseOpenAIResponsesWebSocketEvent(raw)
	require.NoError(t, err)
	round, payload, apiErr := prepareResponsesWebSocketRound(c, info, event, raw, 1)
	require.Nil(t, apiErr)
	require.Equal(t, "request-test-1", round.RequestId)

	var output map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(payload, &output))
	require.NotContains(t, output, "stream")
	require.JSONEq(t, `false`, string(output["generate"]))
	require.JSONEq(t, `0`, string(output["max_output_tokens"]))
	require.JSONEq(t, `0`, string(output["temperature"]))
	require.JSONEq(t, `"stream_1"`, string(output["stream_id"]))
	require.JSONEq(t, `"resp_1"`, string(output["previous_response_id"]))
	require.JSONEq(t, `"evt_1"`, string(output["client_event_id"]))
}

func TestResponsesWebSocketRoundRequestIDLimitedTo64Runes(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "ascii", base: strings.Repeat("a", 80)},
		{name: "multibyte", base: strings.Repeat("界", 80)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestID := responsesWebSocketRoundRequestID(test.base, 123)
			require.Len(t, []rune(requestID), 64)
			require.True(t, strings.HasSuffix(requestID, "-123"))
		})
	}
}

func TestDialResponsesWebSocketUpstreamRejectsExplicitDisable(t *testing.T) {
	c, info := newResponsesWebSocketTestContext(t)
	disabled := false
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{SupportsResponsesWebSocket: &disabled})

	_, apiErr := DialResponsesWebSocketUpstream(c, info)

	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeChannelResponsesWebSocketUnsupported, apiErr.GetErrorCode())
}

func TestResponsesWebSocketIncompleteEventReleasesRound(t *testing.T) {
	info := &relaycommon.RelayInfo{ResponsesWebSocketState: responsesWebSocketStateGenerating}
	session := &responsesWebSocketSession{
		connectionInfo: info,
		state:          responsesWebSocketStateGenerating,
	}

	session.handleUpstreamEvent(dto.ResponsesStreamResponse{Type: "response.incomplete"})

	require.Equal(t, responsesWebSocketStateIdle, session.state)
	require.Equal(t, responsesWebSocketStateIdle, info.ResponsesWebSocketState)
	require.Equal(t, time.Hour, responsesWebSocketMaxLifetime)
}

func TestResponsesWebSocketWarmupConsumptionRequiresUsage(t *testing.T) {
	require.False(t, shouldRecordResponsesWebSocketConsumption(responsesWebSocketStateWarming, &dto.Usage{}))
	require.True(t, shouldRecordResponsesWebSocketConsumption(responsesWebSocketStateWarming, &dto.Usage{TotalTokens: 1}))
	require.True(t, shouldRecordResponsesWebSocketConsumption(responsesWebSocketStateGenerating, &dto.Usage{}))
}

func TestResponsesWebSocketTTFTStartsAtFirstToken(t *testing.T) {
	_, round := newResponsesWebSocketTestContext(t)
	round.StartTime = time.Now().Add(-time.Second)
	round.FirstResponseTime = round.StartTime.Add(-time.Second)
	session := &responsesWebSocketSession{round: round}

	for _, event := range []dto.ResponsesStreamResponse{
		{Type: "response.created"},
		{Type: "response.in_progress"},
		{Type: "response.output_item.added"},
		{Type: "response.output_text.delta"},
		{Type: "response.audio.delta", Delta: "base64-audio-bytes"},
	} {
		session.observeFirstToken(event)
	}
	require.False(t, round.FirstResponseTime.After(round.StartTime))

	session.observeFirstToken(dto.ResponsesStreamResponse{
		Type:  "response.output_text.delta",
		Delta: "first token",
	})
	require.True(t, round.FirstResponseTime.After(round.StartTime))

	firstTokenTime := round.FirstResponseTime
	session.observeFirstToken(dto.ResponsesStreamResponse{
		Type:  "response.function_call_arguments.delta",
		Delta: `{"location":`,
	})
	require.Equal(t, firstTokenTime, round.FirstResponseTime)
}

func TestMergeResponsesWebSocketEnvelopePreservesExplicitEmptyStreamID(t *testing.T) {
	raw := []byte(`{"type":"response.create","model":"gpt-4o","stream_id":""}`)
	event, err := dto.ParseOpenAIResponsesWebSocketEvent(raw)
	require.NoError(t, err)

	payload, err := mergeResponsesWebSocketEnvelope([]byte(`{"model":"gpt-4o"}`), raw, event)
	require.NoError(t, err)

	var output map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(payload, &output))
	require.Contains(t, output, "stream_id")
	require.JSONEq(t, `""`, string(output["stream_id"]))
}

func TestResponsesWebSocketSequentialWarmupRounds(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	var receivedMu sync.Mutex
	received := make([]map[string]json.RawMessage, 0, 2)
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		for i := 0; i < 2; i++ {
			_, payload, readErr := conn.ReadMessage()
			require.NoError(t, readErr)
			var event map[string]json.RawMessage
			require.NoError(t, common.Unmarshal(payload, &event))
			receivedMu.Lock()
			received = append(received, event)
			receivedMu.Unlock()
			require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.failed","response":{"status":"failed"}}`)))
		}
	}))
	defer upstreamServer.Close()

	gateway := gin.New()
	gateway.GET("/v1/responses", func(c *gin.Context) {
		client, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		require.NoError(t, err)
		defer client.Close()
		_, first, err := client.ReadMessage()
		require.NoError(t, err)

		upstreamURL := "ws" + strings.TrimPrefix(upstreamServer.URL, "http")
		upstream, _, err := websocket.DefaultDialer.Dial(upstreamURL, nil)
		require.NoError(t, err)
		common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
		common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
		common.SetContextKey(c, constant.ContextKeyChannelId, 1)
		common.SetContextKey(c, constant.ContextKeyChannelName, "test")
		common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstreamServer.URL)
		common.SetContextKey(c, constant.ContextKeyChannelKey, "upstream-key")
		common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
		common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{})
		common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]interface{}{})
		common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, map[string]interface{}{})
		common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{AcceptUnsetRatioModel: true})
		info := relaycommon.GenRelayInfoResponsesWebSocket(c, &dto.OpenAIResponsesRequest{Model: "gpt-4o"}, client)
		info.RequestId = "request-integration"
		info.TargetWs = upstream
		_ = RunResponsesWebSocket(c, info, first)
	})
	server := httptest.NewServer(gateway)
	defer server.Close()

	clientURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/responses"
	client, _, err := websocket.DefaultDialer.Dial(clientURL, nil)
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-4o","generate":false,"input":[]}`)))
	_, firstResponse, err := client.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(firstResponse), "response.failed")
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","generate":false,"previous_response_id":"resp_1","input":[]}`)))
	_, secondResponse, err := client.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(secondResponse), "response.failed")
	_ = client.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"), time.Now().Add(time.Second))

	require.Eventually(t, func() bool {
		receivedMu.Lock()
		defer receivedMu.Unlock()
		return len(received) == 2
	}, time.Second, 10*time.Millisecond)
	receivedMu.Lock()
	defer receivedMu.Unlock()
	require.JSONEq(t, `false`, string(received[0]["generate"]))
	require.JSONEq(t, `"resp_1"`, string(received[1]["previous_response_id"]))
	require.JSONEq(t, `"gpt-4o"`, string(received[1]["model"]))
}
