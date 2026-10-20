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
