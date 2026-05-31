package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatUserLogsHidesErrorDetails(t *testing.T) {
	logs := []*Log{
		{
			Id:      99,
			Type:    LogTypeError,
			Content: "status_code=401, invalid API key: sk-secret",
			Other:   `{"status_code":401,"error_code":"invalid_api_key","error_type":"openai_error","channel_id":7,"admin_info":{"upstream_error_body":"secret upstream body"}}`,
		},
	}

	formatUserLogs(logs, 0)

	require.Equal(t, 1, logs[0].Id)
	require.Equal(t, "status_code=401", logs[0].Content)
	require.Contains(t, logs[0].Other, `"status_code":401`)
	require.Contains(t, logs[0].Other, `"error_code":"invalid_api_key"`)
	require.NotContains(t, logs[0].Other, "admin_info")
	require.NotContains(t, logs[0].Other, "upstream_error_body")
	require.NotContains(t, logs[0].Other, "channel_id")
}

func TestFormatUserLogsFallsBackToStatusCodeInContent(t *testing.T) {
	logs := []*Log{
		{
			Type:    LogTypeError,
			Content: "status_code=429, rate limit exceeded",
			Other:   "",
		},
	}

	formatUserLogs(logs, 0)

	require.Equal(t, "status_code=429", logs[0].Content)
	require.Equal(t, "{}", logs[0].Other)
}
