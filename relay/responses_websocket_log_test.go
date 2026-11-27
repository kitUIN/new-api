package relay

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestResponsesWebSocketDisconnectStatus(t *testing.T) {
	for _, tc := range []struct {
		peer, want string
		closeCode  int
	}{
		{"client", "user_interrupted", websocket.CloseNormalClosure},
		{"upstream", "stream_interrupted", websocket.CloseNormalClosure},
		{"protocol error", "interrupted", websocket.CloseProtocolError},
	} {
		t.Run(tc.peer, func(t *testing.T) {
			s, client, provider, db := newSteeringTestSession(t)
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = RunResponsesWebSocket(s.c, s.connectionInfo, []byte(`{"type":"response.create","model":"gpt-4o","input":"Hello"}`))
			}()
			t.Cleanup(func() {
				_ = client.Close()
				_ = provider.Close()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("relay did not stop")
				}
			})
			readSteeringFrame(t, provider)
			peer := client
			if tc.peer == "upstream" {
				peer = provider
			}
			require.NoError(t, peer.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(tc.closeCode, ""), time.Now().Add(time.Second)))
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("round did not finish on disconnect")
			}
			var log model.Log
			require.NoError(t, db.First(&log).Error)
			require.Equal(t, tc.want, log.Status)
			require.Zero(t, log.Quota)
		})
	}
}

func TestResponsesWebSocketUsageLogsAppearPerRoundBeforeCompletion(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	startSteeringTestRound(t, s, client, provider)
	var first model.Log
	require.NoError(t, db.First(&first).Error)
	require.Equal(t, model.LogTypePending, first.Type)
	require.Equal(t, "request-test-1", first.RequestId)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`)
	require.NoError(t, db.First(&first, first.Id).Error)
	require.Equal(t, model.LogTypeConsume, first.Type)
	require.Equal(t, "completed", first.Status)
	require.Nil(t, s.handleClientFrame(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-4o","input":"Second request"}`)))
	_ = readSteeringFrame(t, provider)
	var logs []model.Log
	require.NoError(t, db.Order("id").Find(&logs).Error)
	require.Len(t, logs, 2)
	require.Equal(t, first.Id, logs[0].Id)
	require.Equal(t, model.LogTypePending, logs[1].Type)
	require.Equal(t, "request-test-2", logs[1].RequestId)
	s.refundRound()
	require.NoError(t, db.Order("id").Find(&logs).Error)
	require.Equal(t, "completed", logs[0].Status)
	require.Equal(t, "interrupted", logs[1].Status)
	require.Zero(t, logs[1].Quota)
}

func TestResponsesWebSocketUpstream502EventStatus(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	startSteeringTestRound(t, s, client, provider)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"error","status":502,"error":{"message":"Upstream response stream was interrupted"}}`)
	var log model.Log
	require.NoError(t, db.First(&log).Error)
	require.Equal(t, "stream_interrupted", log.Status)
	require.Equal(t, "status_code=502, Upstream response stream was interrupted", log.Content)
}
