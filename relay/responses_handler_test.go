package relay

import (
	"bytes"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRemoveResponsesToolsPreservesNoOpAndRawValues(t *testing.T) {
	for _, input := range []string{
		`{ "input": {"id":9007199254740993,"value":0,"flag":false} }`,
		`{ "input": "hi", "tools": [{"type":"function","name":"keep"}] }`,
		`{ "input": "hi", "tools": null }`,
	} {
		body := []byte(input)
		out, err := removeResponsesImageGenerationToolsFromBody(body)
		require.NoError(t, err)
		require.True(t, bytes.Equal(body, out))
		require.True(t, &body[0] == &out[0])
	}
	input := []byte(`{"tools":[{"type":"image_generation"}],"input":{"id":9007199254740993,"value":0,"flag":false}}`)
	out, err := removeResponsesImageGenerationToolsFromBody(input)
	require.NoError(t, err)
	require.Equal(t, gjson.GetBytes(input, "input").Raw, gjson.GetBytes(out, "input").Raw)
	require.False(t, gjson.GetBytes(out, "tools").Exists())
	duplicate := []byte(`{"tools":[{"type":"function","name":"first"}],"tools":[{"type":"image_generation"}]}`)
	out, err = removeResponsesImageGenerationToolsFromBody(duplicate)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(out, "tools").Exists(), "filter the last duplicate key, matching JSON decoding")
	_, err = removeResponsesImageGenerationToolsFromBody([]byte(`{"tools":`))
	require.Error(t, err)
}

func TestGetFilteredResponsesToolTypesWebSearchDefaultsToEnabled(t *testing.T) {
	require.Equal(t,
		[]string{dto.OpenAIResponsesToolTypeImageGeneration},
		getFilteredResponsesToolTypes(dto.ChannelOtherSettings{}),
	)

	require.Equal(t,
		[]string{
			dto.OpenAIResponsesToolTypeImageGeneration,
			dto.OpenAIResponsesToolTypeWebSearch,
		},
		getFilteredResponsesToolTypes(dto.ChannelOtherSettings{DisableResponsesWebSearchTool: true}),
	)
}

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

func TestRemoveResponsesWebSearchToolsFromBody(t *testing.T) {
	input := []byte(`{
		"model":"deepseek-v4-flash",
		"input":"hi",
		"unknown_field":{"keep":true},
		"tools":[
			{"type":"function","name":"shell_command"},
			{"type":"web_search","external_web_access":false},
			{"type":"web_search_preview","search_context_size":"medium"}
		]
	}`)

	output, err := removeResponsesWebSearchToolsFromBody(input)
	require.NoError(t, err)

	require.False(t, gjson.GetBytes(output, `tools.#[type="web_search"]`).Exists())
	require.Equal(t, "function", gjson.GetBytes(output, "tools.0.type").String())
	require.Equal(t, "web_search_preview", gjson.GetBytes(output, "tools.1.type").String())
	require.True(t, gjson.GetBytes(output, "unknown_field.keep").Bool())
}

func TestRemoveResponsesWebSearchToolsFromBodyDeletesToolsWhenEmpty(t *testing.T) {
	input := []byte(`{
		"model":"deepseek-v4-flash",
		"input":"hi",
		"tools":[{"type":"web_search","external_web_access":false}]
	}`)

	output, err := removeResponsesWebSearchToolsFromBody(input)
	require.NoError(t, err)

	require.False(t, gjson.GetBytes(output, "tools").Exists())
	require.Equal(t, "deepseek-v4-flash", gjson.GetBytes(output, "model").String())
}
