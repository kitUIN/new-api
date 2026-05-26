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
