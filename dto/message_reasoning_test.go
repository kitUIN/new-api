package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMessageReasoningPreservesExplicitEmptyStrings(t *testing.T) {
	raw := []byte(`{"role":"assistant","content":"Hello","reasoning_content":"","reasoning":""}`)

	var msg Message
	require.NoError(t, common.Unmarshal(raw, &msg))
	require.NotNil(t, msg.ReasoningContent)
	require.NotNil(t, msg.Reasoning)

	encoded, err := common.Marshal(msg)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(encoded, "reasoning_content").Exists())
	require.True(t, gjson.GetBytes(encoded, "reasoning").Exists())
}

func TestMessageReasoningOmitsAbsentFields(t *testing.T) {
	var msg Message
	require.NoError(t, common.Unmarshal([]byte(`{"role":"assistant","content":"Hello"}`), &msg))
	require.Nil(t, msg.ReasoningContent)
	require.Nil(t, msg.Reasoning)

	encoded, err := common.Marshal(msg)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(encoded, "reasoning_content").Exists())
	require.False(t, gjson.GetBytes(encoded, "reasoning").Exists())
}

func TestMessageGetReasoningContent(t *testing.T) {
	reasoningContent := "primary"
	reasoning := "fallback"
	require.Equal(t, "primary", (&Message{ReasoningContent: &reasoningContent, Reasoning: &reasoning}).GetReasoningContent())
	require.Equal(t, "fallback", (&Message{Reasoning: &reasoning}).GetReasoningContent())
	require.Equal(t, "", (&Message{}).GetReasoningContent())
}
