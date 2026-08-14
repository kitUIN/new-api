package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/require"
)

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
