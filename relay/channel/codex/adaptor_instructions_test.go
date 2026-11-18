package codex

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestCodexChannelInstructions(t *testing.T) {
	const developer = `{"role":"developer","content":"site"}`
	const user = `{"role":"user","content":"hi"}`
	const tool = `{"type":"function_call_output","call_id":"c","output":"ok","extension":9007199254740993}`
	for _, test := range []struct {
		name, model, input, instructions string
		override                         bool
		wantInput, wantInstructions      string
	}{
		{"string input", "gpt-6.2-sol", `"hi"`, "", false, `[` + developer + `,` + user + `]`, `""`},
		{"turn override", "gpt-6.2-astra", `[` + tool + `]`, `"this turn"`, true, `[` + developer + `,` + tool + `]`, `"this turn"`},
		{"keep client instructions", "gpt-6.2-sol", `"hi"`, `"this turn"`, false, `"hi"`, `"this turn"`},
		{"no input", "gpt-6.2-sol", "", "", false, `[` + developer + `]`, `""`},
		{"legacy override", "gpt-6.1-sol", `"hi"`, `"this turn"`, true, `"hi"`, `"site\nthis turn"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses, ChannelMeta: &relaycommon.ChannelMeta{
				ChannelSetting: dto.ChannelSettings{SystemPrompt: "site", SystemPromptOverride: test.override},
			}}
			result, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
				Model: test.model, Input: []byte(test.input), Instructions: []byte(test.instructions),
			})
			require.NoError(t, err)
			request := result.(dto.OpenAIResponsesRequest)
			require.JSONEq(t, test.wantInput, string(request.Input))
			require.Equal(t, test.wantInstructions, string(request.Instructions))
			if test.input == `[`+tool+`]` {
				require.Contains(t, string(request.Input), "9007199254740993")
			}
		})
	}
}
