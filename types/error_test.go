package types

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestLimitErrorMessageForResponseUsesRuneCount(t *testing.T) {
	exactlyAtLimit := strings.Repeat("错", MaxErrorMessageRunes)
	require.Equal(t, exactlyAtLimit, LimitErrorMessageForResponse(exactlyAtLimit))

	overLimit := exactlyAtLimit + "误"
	limited := LimitErrorMessageForResponse(overLimit)
	require.Equal(t, MaxErrorMessageRunes, utf8.RuneCountInString(limited))
	require.True(t, utf8.ValidString(limited))
	require.True(t, strings.HasSuffix(limited, errorMessageOmittedSuffix))
}

func TestResponseErrorsAreLimitedWithoutMutatingOriginalError(t *testing.T) {
	message := strings.Repeat("upstream error ", 50)
	err := NewOpenAIError(errors.New(message), ErrorCodeBadResponseStatusCode, 502)

	require.Equal(t, message, err.ToOpenAIError().Message)
	require.Equal(t, message, err.Error())
	require.Equal(t, MaxErrorMessageRunes, utf8.RuneCountInString(err.ToOpenAIErrorForResponse().Message))
	require.Equal(t, MaxErrorMessageRunes, utf8.RuneCountInString(err.ToClaudeErrorForResponse().Message))
	require.Equal(t, message, err.Error())
}
