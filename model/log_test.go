package model

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newLogTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = &http.Request{
		Header:     make(http.Header),
		RemoteAddr: "127.0.0.1:12345",
	}
	c.Set("username", "test-user")
	return c
}

func TestRecordConsumeLogMarksZeroTokenUsageAsError(t *testing.T) {
	truncateTables(t)

	RecordConsumeLog(newLogTestContext(), 1, RecordConsumeLogParams{
		ChannelId:        10,
		PromptTokens:     0,
		CompletionTokens: 12,
		ModelName:        "gpt-test",
		Content:          "test",
		Other: map[string]interface{}{
			"request_path": "/v1/chat/completions",
		},
	})

	var log Log
	require.NoError(t, LOG_DB.First(&log).Error)
	require.Equal(t, LogTypeError, log.Type)
	require.Equal(t, 0, log.PromptTokens)
	require.Equal(t, 12, log.CompletionTokens)
	require.Contains(t, log.Other, "zero_token_error")
	require.Contains(t, log.Other, "prompt_tokens")
}

func TestRecordConsumeLogKeepsValidTokenUsageAsConsume(t *testing.T) {
	truncateTables(t)

	RecordConsumeLog(newLogTestContext(), 1, RecordConsumeLogParams{
		ChannelId:        10,
		PromptTokens:     8,
		CompletionTokens: 12,
		ModelName:        "gpt-test",
		Content:          "test",
		Other: map[string]interface{}{
			"request_path": "/v1/chat/completions",
		},
	})

	var log Log
	require.NoError(t, LOG_DB.First(&log).Error)
	require.Equal(t, LogTypeConsume, log.Type)
	require.NotContains(t, log.Other, "zero_token_error")
}

func TestRecordConsumeLogDoesNotMarkNonTokenBillingAsError(t *testing.T) {
	truncateTables(t)

	RecordConsumeLog(newLogTestContext(), 1, RecordConsumeLogParams{
		ChannelId:        10,
		PromptTokens:     0,
		CompletionTokens: 0,
		ModelName:        "mj-imagine",
		Content:          "per call",
		Other: map[string]interface{}{
			"model_price": 0.01,
		},
	})

	var log Log
	require.NoError(t, LOG_DB.First(&log).Error)
	require.Equal(t, LogTypeConsume, log.Type)
}

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
