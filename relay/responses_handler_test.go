package relay

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRemoveResponsesImageGenerationToolsFromBody(t *testing.T) {
	input := []byte(`{
		"model":"gpt-5.5",
		"input":"hi",
		"unknown_field":{"keep":true},
		"tools":[
			{"type":"function","name":"shell_command"},
			{"type":"image_generation","output_format":"png"},
			{"type":"web_search","search_content_types":["text","image"]}
		]
	}`)

	output, err := removeResponsesImageGenerationToolsFromBody(input)
	require.NoError(t, err)

	require.False(t, gjson.GetBytes(output, `tools.#[type="image_generation"]`).Exists())
	require.Equal(t, "function", gjson.GetBytes(output, "tools.0.type").String())
	require.Equal(t, "web_search", gjson.GetBytes(output, "tools.1.type").String())
	require.True(t, gjson.GetBytes(output, "unknown_field.keep").Bool())
}

func TestRemoveResponsesImageGenerationToolsFromBodyDeletesToolsWhenEmpty(t *testing.T) {
	input := []byte(`{
		"model":"gpt-5.5",
		"input":"hi",
		"tools":[{"type":"image_generation","output_format":"png"}]
	}`)

	output, err := removeResponsesImageGenerationToolsFromBody(input)
	require.NoError(t, err)

	require.False(t, gjson.GetBytes(output, "tools").Exists())
	require.Equal(t, "gpt-5.5", gjson.GetBytes(output, "model").String())
}
