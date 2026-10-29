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

func TestResponseMessageOverridesBothFormatsWithoutChangingUpstreamError(t *testing.T) {
	for _, err := range []*NewAPIError{
		NewOpenAIError(errors.New("upstream error"), ErrorCodeBadResponseStatusCode, 503),
		WithClaudeError(ClaudeError{Type: "api_error", Message: "upstream error"}, 503),
	} {
		err.ResponseMessage = strings.Repeat("限额已用光", 150)
		require.Equal(t, LimitErrorMessageForResponse(err.ResponseMessage), err.ToOpenAIErrorForResponse().Message)
		require.Equal(t, LimitErrorMessageForResponse(err.ResponseMessage), err.ToClaudeErrorForResponse().Message)
		require.Equal(t, "upstream error", err.Error())
		require.Equal(t, "upstream error", err.ToOpenAIError().Message)
		require.Equal(t, "upstream error", err.ToClaudeError().Message)
	}
}
