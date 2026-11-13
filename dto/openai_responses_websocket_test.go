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

func TestParseResponsesWebSocketSteer(t *testing.T) {
	for _, input := range []string{
		`"Please also cover failures"`,
		`[{"role":"user","content":"Use Go"}]`,
		`[{"type":"message","role":"user","content":[{"type":"input_text","text":"Check this"},{"type":"input_image","image_url":"https://example.com/image.png","detail":"low"},{"type":"input_file","file_id":"file_1"}]}]`,
	} {
		raw := `{"type":"response.steer","previous_response_id":"resp_1","input":` + input + `}`
		event, err := ParseOpenAIResponsesWebSocketSteerEvent([]byte(raw))
		require.NoError(t, err)
		encoded, err := common.Marshal(event)
		require.NoError(t, err)
		require.JSONEq(t, raw, string(encoded))
		// Steering cannot be the first request used for routing a connection.
		_, err = ParseOpenAIResponsesWebSocketEvent([]byte(raw))
		require.Error(t, err)
	}
}

func TestParseResponsesWebSocketSteerRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{
		`null`, `0`, `false`, `{}`, `[]`, `[null]`, `["text"]`,
		`[{"role":"assistant","content":"text"}]`,
		`[{"role":"user","type":"function_call_output","content":"text"}]`,
		`[{"role":"user","content":null}]`,
		`[{"role":"user","content":[{"type":"output_text","text":"text"}]}]`,
		`[{"role":"user","id":"item_1","content":"text"}]`,
	} {
		t.Run(input, func(t *testing.T) {
			_, err := ParseOpenAIResponsesWebSocketSteerEvent([]byte(`{"type":"response.steer","previous_response_id":"resp_1","input":` + input + `}`))
			require.Error(t, err)
		})
	}
	for _, raw := range []string{
		`{"type":"response.steer","input":"text"}`,
		`{"type":"response.steer","previous_response_id":"resp_1","input":"text","model":"other"}`,
		`{"type":"response.steer","previous_response_id":"resp_1","input":"text","stream_id":"main"}`,
	} {
		_, err := ParseOpenAIResponsesWebSocketSteerEvent([]byte(raw))
		require.Error(t, err)
	}
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
