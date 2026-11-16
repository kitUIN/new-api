package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecordLogsPreservesRequestClient(t *testing.T) {
	for _, logType := range []int{LogTypeConsume, LogTypeError} {
		truncateTables(t)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
		ctx.Request.Header.Set("User-Agent", "Codex Desktop/0.169.0")
		ctx.Request.Header.Set("Authorization", "Bearer never-store-this")
		ctx.Request.Header.Set("Cookie", "session=never-store-cookie")
		ctx.Request.Header.Set("X-Custom", "never-store-custom-header")
		if logType == LogTypeConsume {
			RecordConsumeLog(ctx, 42, RecordConsumeLogParams{})
		} else {
			RecordErrorLog(ctx, 42, 7, "test-model", "", "error", 0, 1, false, "", nil)
		}
		logs, _, err := GetUserLogs(42, logType, 0, 0, "", "", 0, 10, "", "", "")
		require.NoError(t, err)
		require.Len(t, logs, 1)
		var other map[string]interface{}
		require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
		require.Equal(t, "codex-desktop", other["request_client"])
		require.Equal(t, "Codex Desktop/0.169.0", other["user_agent"])
		require.NotContains(t, other, "request_headers")
		require.NotContains(t, logs[0].Other, "never-store-this")
		require.NotContains(t, logs[0].Other, "never-store-cookie")
		require.NotContains(t, logs[0].Other, "never-store-custom-header")
		ctx.Request.Header.Set("User-Agent", "undici")
		require.NotContains(t, appendRequestLogInfo(ctx, other), "request_client")
		require.Equal(t, "undici", other["user_agent"])
		ctx.Request.Header.Del("User-Agent")
		require.NotContains(t, appendRequestLogInfo(ctx, other), "user_agent")
	}
}

func TestRequestClientOptionRejectsInvalidUpdate(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})
	require.NoError(t, DB.AutoMigrate(&Option{}))
	t.Cleanup(func() { require.NoError(t, UpdateOption("RequestClientRules", common.DefaultRequestClientRules)) })
	require.NoError(t, UpdateOption("RequestClientRules", `[]`))
	require.Error(t, UpdateOption(" requestclientrules ", `[{}]`))
	require.Error(t, UpdateOption("RequestClientRules", `[{}]`))
	var option Option
	require.NoError(t, DB.Where("key = ?", "RequestClientRules").First(&option).Error)
	require.Equal(t, `[]`, option.Value)
	require.Empty(t, common.GetRequestClientDisplay())
}
