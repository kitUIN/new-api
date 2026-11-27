package model

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

func startTestUsageLog(t *testing.T, requestID string) (*gin.Context, *relaycommon.RelayInfo, int) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set("username", "test")
	c.Set(common.RequestIdKey, requestID)
	info := &relaycommon.RelayInfo{UserId: 42, TokenId: 7, RequestId: requestID,
		OriginModelName: "test-model", StartTime: time.Now(), UsingGroup: "default"}
	BeginRequestLog(c, info, map[string]interface{}{"request_path": "/v1/responses"})
	id := pendingLog(c).log.Id
	t.Cleanup(func() { activeUsageLogs.Delete(id) })
	return c, info, id
}

func TestUsageLogVisibleBeforeSettlementAndUpdatedInPlace(t *testing.T) {
	truncateTables(t)
	c, info, id := startTestUsageLog(t, "request-1")
	logs, total, err := GetUserLogs(42, 0, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, LogTypePending, logs[0].Type)
	require.Equal(t, "in_progress", logs[0].Status)
	created := logs[0].CreatedAt
	stat, err := SumUsedQuota(0, 0, 0, "", "test", "", 0, "", "")
	require.NoError(t, err)
	require.Zero(t, stat.Rpm)
	require.Zero(t, stat.Quota)
	_, total, err = GetUserLogs(99, 0, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.Zero(t, total)
	RecordConsumeLog(c, 42, RecordConsumeLogParams{PromptTokens: 100, CompletionTokens: 20, Quota: 30, ModelName: "test-model"})
	FinishRequestLog(c, info, nil)
	var completed []Log
	require.NoError(t, LOG_DB.Find(&completed).Error)
	require.Len(t, completed, 1)
	require.Equal(t, id, completed[0].Id)
	require.Equal(t, created, completed[0].CreatedAt)
	require.Equal(t, LogTypeConsume, completed[0].Type)
	require.Equal(t, "completed", completed[0].Status)
	require.Equal(t, 30, completed[0].Quota)
	require.Equal(t, 20, completed[0].CompletionTokens)
	require.Positive(t, completed[0].FinishedAt)
	stat, err = SumUsedQuota(0, 0, 0, "", "test", "", 0, "", "")
	require.NoError(t, err)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 30, stat.Quota)
}

func TestUsageLogRetriesDoNotCompleteOrDuplicateRequest(t *testing.T) {
	truncateTables(t)
	c, info, id := startTestUsageLog(t, "request-2")
	message := "status_code=502, Upstream HTTP/2 stream failed"
	RecordErrorLog(c, 42, 7, "test-model", "", message, 7, 1, true, "default", nil)
	var pending Log
	require.NoError(t, LOG_DB.First(&pending, id).Error)
	require.Equal(t, LogTypePending, pending.Type)
	RecordConsumeLog(c, 42, RecordConsumeLogParams{ChannelId: 8, Quota: 0, IsStream: false})
	FinishRequestLog(c, info, nil)
	var logs []Log
	require.NoError(t, LOG_DB.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, 8, logs[0].ChannelId)
	require.Zero(t, logs[0].Quota)
	require.False(t, logs[0].IsStream)
	require.Equal(t, "completed", logs[0].Status)
	attempt := gjson.Get(logs[0].Other, "admin_info.retry_errors.0")
	require.Equal(t, message, attempt.Get("message").String())
	require.Equal(t, "stream_interrupted", attempt.Get("status").String())
}

func TestUsageLogFailureDisconnectAndStreamInterruption(t *testing.T) {
	for _, tc := range []struct {
		name, message, endReason, want string
		cancelRequest                  bool
	}{
		{name: "unattributed", want: "interrupted"},
		{name: "user cancels HTTP request", cancelRequest: true, want: "user_interrupted"},
		{name: "unrelated 502", message: "Bad Gateway", want: "failed"},
		{name: "HTTP2 failure", message: "Upstream HTTP/2 stream failed", want: "stream_interrupted"},
		{name: "upstream adds request ID", message: "Upstream HTTP/2 stream failed (request id: abc)", want: "stream_interrupted"},
		{name: "upstream interruption", message: "Upstream response stream was interrupted", want: "stream_interrupted"},
		{name: "client closes SSE", endReason: "client_gone", want: "user_interrupted"},
		{name: "SSE read failure", endReason: "scanner_error", want: "stream_interrupted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)
			c, info, id := startTestUsageLog(t, tc.name)
			var apiErr *types.NewAPIError
			if tc.message != "" {
				apiErr = types.NewErrorWithStatusCode(errors.New(tc.message), types.ErrorCodeBadResponse, 502)
			}
			if tc.cancelRequest {
				ctx, cancel := context.WithCancel(c.Request.Context())
				c.Request = c.Request.WithContext(ctx)
				cancel()
			}
			if tc.endReason != "" {
				RecordConsumeLog(c, 42, RecordConsumeLogParams{Quota: 3, Other: map[string]interface{}{
					"stream_status": map[string]interface{}{"status": "error", "end_reason": tc.endReason},
				}})
			}
			FinishRequestLog(c, info, apiErr)
			var log Log
			require.NoError(t, LOG_DB.Session(&gorm.Session{SkipHooks: true}).First(&log, id).Error)
			require.Equal(t, tc.want, log.Status)
			if tc.endReason != "" {
				require.Equal(t, LogTypeConsume, log.Type)
				require.Equal(t, 3, log.Quota)
			} else {
				if tc.want == "failed" {
					require.Equal(t, LogTypeError, log.Type)
				} else {
					require.Equal(t, LogTypeInterrupted, log.Type)
				}
				require.Zero(t, log.Quota)
			}
			require.Positive(t, log.FinishedAt)
		})
	}
}

func TestHistoricalUsageLogInterruptions(t *testing.T) {
	truncateTables(t)
	rows := []Log{
		{Type: LogTypeError, Status: "failed", Content: "status_code=502, Upstream HTTP/2 stream failed"},
		{Type: LogTypeError, Content: "status_code=502, Upstream response stream was interrupted"},
		{Type: LogTypeError, Status: "failed", Content: "status_code=503, Upstream HTTP/2 stream failed"},
		{Type: LogTypeError, Status: "interrupted", Content: "Request interrupted before completion", Other: `{"ws":true}`},
		{Type: LogTypeConsume, Status: "interrupted", Other: `{"stream_status":{"status":"error","end_reason":"client_gone"}}`},
		{Type: LogTypeConsume, Status: "completed", Other: `{"admin_info":{"retry_errors":[{"channel":134,"message":"status_code=502, Upstream response stream was interrupted"}]}}`},
	}
	require.NoError(t, LOG_DB.Create(&rows).Error)
	var logs []Log
	require.NoError(t, LOG_DB.Order("id").Find(&logs).Error)
	for i, status := range []string{"stream_interrupted", "stream_interrupted", "failed", "interrupted", "user_interrupted", "completed"} {
		require.Equal(t, status, logs[i].Status)
	}
	require.Equal(t, "stream_interrupted", gjson.Get(logs[5].Other, "admin_info.retry_errors.0.status").String())
}

func TestUsageLogTypeFiltersIncludeLegacyRowsBeforePagination(t *testing.T) {
	truncateTables(t)
	rows := []Log{
		{UserId: 42, Type: LogTypeError, Content: "status_code=502,  Upstream HTTP/2 stream failed (request id: test)"},
		{UserId: 42, Type: LogTypeError, Content: "status_code=429, rate limited"},
		{UserId: 42, Type: LogTypeError, Content: "status_code=502, Bad Gateway"},
		{UserId: 42, Type: LogTypeInterrupted, Status: logUserInterrupted},
		{UserId: 42, Type: LogTypeConsume, Status: logStreamInterrupted, Quota: 7},
		{UserId: 43, Type: LogTypeRateLimited},
		{UserId: 43, Type: LogTypeInterrupted},
	}
	require.NoError(t, LOG_DB.Create(&rows).Error)
	for _, tc := range []struct{ kind, self, all int }{
		{LogTypeError, 1, 1}, {LogTypeInterrupted, 3, 4}, {LogTypeRateLimited, 1, 2},
	} {
		logs, total, err := GetUserLogs(42, tc.kind, 0, 0, "", "", 0, 1, "", "", "")
		require.NoError(t, err)
		require.EqualValues(t, tc.self, total)
		require.Len(t, logs, 1)
		require.Equal(t, 42, logs[0].UserId)
		logs, total, err = GetAllLogs(tc.kind, 0, 0, "", "", "", 0, 20, 0, "", "", "")
		require.NoError(t, err)
		require.EqualValues(t, tc.all, total)
		require.Len(t, logs, tc.all)
		for _, log := range logs {
			if log.Type != LogTypeConsume {
				require.Equal(t, tc.kind, log.Type)
			}
		}
	}
}

func TestUsageLogRateLimitAfterChannelExhaustion(t *testing.T) {
	truncateTables(t)
	c, info, id := startTestUsageLog(t, "mapped-429")
	info.LastError = types.NewErrorWithStatusCode(errors.New("upstream rate limit"), types.ErrorCodeBadResponse, 503)
	info.LastError.UpstreamStatusCode = 429
	RecordErrorLog(c, 42, 7, "model", "", info.LastError.ErrorWithStatusCode(), 1, 0, false, "default", nil, info)
	FinishRequestLog(c, info, types.NewError(errors.New("no further channels"), types.ErrorCodeGetChannelFailed))
	var log Log
	require.NoError(t, LOG_DB.Session(&gorm.Session{SkipHooks: true}).First(&log, id).Error)
	require.Equal(t, LogTypeRateLimited, log.Type)
	require.Equal(t, info.LastError.ErrorWithStatusCode(), log.Content)
}

func TestUsageLogLeasesPreserveLiveRequestsAcrossNodes(t *testing.T) {
	truncateTables(t)
	c, info, id := startTestUsageLog(t, "local")
	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Model(&Log{}).Where("id = ?", id).Update("heartbeat_at", now-300).Error)
	remote := Log{Type: LogTypePending, Status: "in_progress", HeartbeatAt: now - 10}
	stale := Log{Type: LogTypePending, Status: "in_progress", HeartbeatAt: now - 300}
	require.NoError(t, LOG_DB.Create(&remote).Error)
	require.NoError(t, LOG_DB.Create(&stale).Error)
	require.NoError(t, maintainUsageLogLeases(now))
	for _, liveID := range []int{id, remote.Id} {
		var log Log
		require.NoError(t, LOG_DB.First(&log, liveID).Error)
		require.Equal(t, LogTypePending, log.Type)
	}
	require.NoError(t, LOG_DB.First(&stale, stale.Id).Error)
	require.Equal(t, LogTypeInterrupted, stale.Type)
	require.Equal(t, "interrupted", stale.Status)
	FinishRequestLog(c, info, nil)
}

func TestUsageLogNotificationsAreScopedAndCoalesced(t *testing.T) {
	owner, closeOwner := SubscribeUsageLogs(42)
	defer closeOwner()
	other, closeOther := SubscribeUsageLogs(43)
	defer closeOther()
	admin, closeAdmin := SubscribeUsageLogs(0)
	defer closeAdmin()
	for i := 0; i < 100; i++ {
		NotifyUsageLogChanged(42)
	}
	require.Len(t, owner, 1)
	require.Len(t, admin, 1)
	require.Len(t, other, 0)
}

func TestUsageLogRequestIDCollisionAndWebSocketAdoption(t *testing.T) {
	truncateTables(t)
	first, info, firstID := startTestUsageLog(t, "same-client-id")
	second, _, secondID := startTestUsageLog(t, "same-client-id")
	info.RequestId = "same-client-id-1"
	BeginRequestLog(first, info, nil)
	require.Equal(t, firstID, pendingLog(first).log.Id)
	first.Set(common.RequestIdKey, info.RequestId)
	RecordConsumeLog(first, 42, RecordConsumeLogParams{Quota: 5})
	RecordConsumeLog(second, 42, RecordConsumeLogParams{Quota: 9})
	var logs []Log
	require.NoError(t, LOG_DB.Order("id").Find(&logs).Error)
	require.Len(t, logs, 2)
	require.Equal(t, firstID, logs[0].Id)
	require.Equal(t, secondID, logs[1].Id)
	require.Equal(t, 5, logs[0].Quota)
	require.Equal(t, 9, logs[1].Quota)
}

func TestUsageLogDisabledAndWarmupDoNotCreatePlaceholders(t *testing.T) {
	truncateTables(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{ResponsesWebSocketWarmup: true}
	BeginRequestLog(c, info, nil)
	require.Nil(t, pendingLog(c))
	original := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = original })
	info.ResponsesWebSocketWarmup = false
	BeginRequestLog(c, info, nil)
	require.Nil(t, pendingLog(c))
}
