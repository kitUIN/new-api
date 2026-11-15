package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateChannelClaudeCodeSetting(t *testing.T) {
	for _, settings := range []string{"", `{}`, `{"disable_claude_code":true}`, `{"disable_claude_code":false}`} {
		channel := &Channel{OtherSettings: settings}
		require.NoError(t, channel.ValidateSettings())
	}
	for _, settings := range []string{`{"disable_claude_code":"true"}`, `{"disable_claude_code":1}`, `{"disable_claude_code":[]}`} {
		channel := &Channel{OtherSettings: settings}
		require.Error(t, channel.ValidateSettings())
	}
}

func TestValidateChannelClientRestrictions(t *testing.T) {
	for _, settings := range []string{
		`{"client_restrictions":{}}`,
		`{"client_restrictions":null}`,
		`{"client_restrictions":{"codex":"allow_only","claude_code":"deny"}}`,
		`{"client_restrictions":{"codex":"deny","claude_code":"allow_only"}}`,
		`{"client_restrictions":{"codex":"unrestricted","claude_code":""}}`,
	} {
		require.NoError(t, (&Channel{OtherSettings: settings}).ValidateSettings(), settings)
	}
	for _, settings := range []string{
		`{"client_restrictions":true}`,
		`{"client_restrictions":[]}`,
		`{"client_restrictions":"deny"}`,
		`{"client_restrictions":{"codex":"unknown"}}`,
		`{"client_restrictions":{"claude_code":"ALLOW_ONLY"}}`,
		`{"client_restrictions":{"codex":true}}`,
		`{"client_restrictions":{"claude_code":[]}}`,
	} {
		require.Error(t, (&Channel{OtherSettings: settings}).ValidateSettings(), settings)
	}
}
