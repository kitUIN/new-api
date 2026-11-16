package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

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
