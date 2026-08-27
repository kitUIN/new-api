package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestParseOpenAIResponsesWebSocketEventPreservesExplicitFalseAndZero(t *testing.T) {
	event, err := ParseOpenAIResponsesWebSocketEvent([]byte(`{
  "type":"response.create",
  "model":"gpt-5.2-codex",
  "generate":false,
  "stream_id":"stream_1",
  "max_output_tokens":0,
  "temperature":0
}`))
	require.NoError(t, err)
	require.NotNil(t, event.Generate)
	require.False(t, *event.Generate)
	require.NotNil(t, event.StreamID)
	require.Equal(t, "stream_1", *event.StreamID)
	require.NotNil(t, event.MaxOutputTokens)
	require.Zero(t, *event.MaxOutputTokens)
	require.NotNil(t, event.Temperature)
	require.Zero(t, *event.Temperature)

	encoded, err := common.Marshal(event)
	require.NoError(t, err)
	require.JSONEq(t, `{
  "type":"response.create",
  "model":"gpt-5.2-codex",
  "generate":false,
  "stream_id":"stream_1",
  "max_output_tokens":0,
  "temperature":0
}`, string(encoded))
}

func TestParseOpenAIResponsesWebSocketEventPreservesExplicitEmptyStreamID(t *testing.T) {
	event, err := ParseOpenAIResponsesWebSocketEvent([]byte(`{
  "type":"response.create",
  "model":"gpt-5.2-codex",
  "stream_id":""
}`))
	require.NoError(t, err)
	require.NotNil(t, event.StreamID)
	require.Empty(t, *event.StreamID)

	encoded, err := common.Marshal(event)
	require.NoError(t, err)
	require.JSONEq(t, `{
  "type":"response.create",
  "model":"gpt-5.2-codex",
  "stream_id":""
}`, string(encoded))
}

func TestHasOpenAIResponsesWebSocketBackgroundRejectsAnyPresence(t *testing.T) {
	present, err := HasOpenAIResponsesWebSocketBackground([]byte(`{"type":"response.create"}`))
	require.NoError(t, err)
	require.False(t, present)

	present, err = HasOpenAIResponsesWebSocketBackground([]byte(`{"type":"response.create","background":false}`))
	require.NoError(t, err)
	require.True(t, present)
}

func TestParseOpenAIResponsesWebSocketEventRejectsOtherTypes(t *testing.T) {
	_, err := ParseOpenAIResponsesWebSocketEvent([]byte(`{"type":"session.update","model":"gpt-5.2-codex"}`))
	require.ErrorContains(t, err, "response.create")
}

func TestResponsesWebSocketChannelCapabilityDefaultsOn(t *testing.T) {
	settings := ChannelOtherSettings{}
	require.True(t, settings.IsResponsesWebSocketEnabled())
	encoded, err := common.Marshal(settings)
	require.NoError(t, err)
	var output map[string]any
	require.NoError(t, common.Unmarshal(encoded, &output))
	require.NotContains(t, output, "supports_responses_websocket")

	disabled := false
	settings.SupportsResponsesWebSocket = &disabled
	require.False(t, settings.IsResponsesWebSocketEnabled())
	encoded, err = common.Marshal(settings)
	require.NoError(t, err)
	output = nil
	require.NoError(t, common.Unmarshal(encoded, &output))
	require.Equal(t, false, output["supports_responses_websocket"])
}
