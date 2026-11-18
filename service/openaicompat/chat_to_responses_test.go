package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/require"
)

func TestChatToResponsesInstructionPlacement(t *testing.T) {
	var request dto.GeneralOpenAIRequest
	require.NoError(t, common.UnmarshalJsonStr(`{"messages":[
		{"role":"system","content":" base "},
		{"role":"user","content":"hello"},
		{"role":"developer","content":[{"type":"text","text":"later"},{"type":"text","text":"update"}]},
		{"role":"tool","tool_call_id":"call_1","content":"result"}
	]}`, &request))
	for _, model := range []string{"gpt-6.2-sol", "gpt-6.2-astra-2026-11-18", "openai/gpt-6.10-sol", "gpt-7"} {
		t.Run(model, func(t *testing.T) {
			request.Model = model
			converted, err := ChatCompletionsRequestToResponsesRequest(&request)
			require.NoError(t, err)
			require.Empty(t, converted.Instructions)
			require.JSONEq(t, `[{"role":"developer","content":" base "},{"role":"user","content":"hello"},{"role":"developer","content":"later\nupdate"},{"type":"function_call_output","call_id":"call_1","output":"result"}]`, string(converted.Input))
		})
	}
	for _, model := range []string{"gpt-6.1-sol", "gpt-6-astra", "gpt-5.4", "gpt-6.2x", "other/gpt-6.2"} {
		t.Run(model, func(t *testing.T) {
			request.Model = model
			converted, err := ChatCompletionsRequestToResponsesRequest(&request)
			require.NoError(t, err)
			require.JSONEq(t, `"base\n\nlater\nupdate"`, string(converted.Instructions))
			require.JSONEq(t, `[{"role":"user","content":"hello"},{"type":"function_call_output","call_id":"call_1","output":"result"}]`, string(converted.Input))
		})
	}
}

func TestChatCompletionsRequestToResponsesRequestPreservesServiceTier(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "fast", raw: []byte(`"fast"`), want: "fast"},
		{name: "priority alias", raw: []byte(`"priority"`), want: "priority"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
				Model:       "gpt-5.6-sol",
				ServiceTier: tt.raw,
			})
			require.NoError(t, err)
			require.Equal(t, tt.want, converted.ServiceTier)
		})
	}
}

func TestChatCompletionsRequestToResponsesRequestRejectsInvalidServiceTier(t *testing.T) {
	_, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
		Model:       "gpt-5.6-sol",
		ServiceTier: []byte(`false`),
	})

	require.ErrorContains(t, err, "invalid service_tier")
}
