package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProcessChannelErrorRecordsCompleteRawUpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalLogDB := model.LOG_DB
	originalErrorLogEnabled := constant.ErrorLogEnabled
	db, err := gorm.Open(sqlite.Open("file:relay_error_log?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	model.LOG_DB = db
	constant.ErrorLogEnabled = true
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		constant.ErrorLogEnabled = originalErrorLogEnabled
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", 123)
	c.Set("username", "raw-upstream-test")
	c.Set("channel_id", 7)
	c.Set("channel_name", "upstream")
	c.Set("channel_type", 1)

	rawBody := `{"error":{"message":"https://api.example.com/private/` + strings.Repeat("错误", 2500) + `"}}`
	apiErr := types.NewOpenAIError(
		errors.New("upstream failed"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)
	apiErr.ResponseBody = rawBody

	processChannelError(c, *types.NewChannelError(7, 1, "upstream", false, "", false), apiErr)

	var log model.Log
	require.NoError(t, db.First(&log).Error)
	var other map[string]any
	require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
	adminInfo, ok := other["admin_info"].(map[string]any)
	require.True(t, ok)
	storedBody, ok := adminInfo["upstream_error_body"].(string)
	require.True(t, ok)
	require.LessOrEqual(t, len(storedBody), maxUpstreamErrorBodyBytes)
	require.True(t, utf8.ValidString(storedBody))
	require.True(t, strings.HasSuffix(storedBody, "..."))
	require.Contains(t, storedBody, "https://api.example.com/private/")
	require.Greater(t, len(rawBody), maxUpstreamErrorBodyBytes)
}

func TestTruncateLogFieldIncludesSuffixInByteLimit(t *testing.T) {
	value := strings.Repeat("a", maxUpstreamErrorBodyBytes+1)
	truncated := truncateLogField(value, maxUpstreamErrorBodyBytes)

	require.Len(t, truncated, maxUpstreamErrorBodyBytes)
	require.True(t, strings.HasSuffix(truncated, "..."))
	require.Equal(t, "原样返回", truncateLogField("原样返回", maxUpstreamErrorBodyBytes))
}
