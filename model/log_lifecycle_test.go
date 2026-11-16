package model

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
	RecordErrorLog(c, 42, 7, "test-model", "", "upstream busy", 7, 1, true, "default", nil)
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
	require.Contains(t, logs[0].Other, "upstream busy")
	require.Contains(t, logs[0].Other, "retry_errors")
}

func TestUsageLogFailureDisconnectAndStreamInterruption(t *testing.T) {
	for _, state := range []string{"failed", "interrupted", "stream"} {
		t.Run(state, func(t *testing.T) {
			truncateTables(t)
			c, info, id := startTestUsageLog(t, state)
			var apiErr *types.NewAPIError
			if state == "failed" {
				apiErr = types.NewError(errors.New("bad request"), types.ErrorCodeInvalidRequest)
			}
			if state == "stream" {
				RecordConsumeLog(c, 42, RecordConsumeLogParams{Quota: 3, Other: map[string]interface{}{"stream_status": map[string]interface{}{"status": "error"}}})
			}
			FinishRequestLog(c, info, apiErr)
			var log Log
			require.NoError(t, LOG_DB.First(&log, id).Error)
			if state == "failed" {
				require.Equal(t, "failed", log.Status)
			} else {
				require.Equal(t, "interrupted", log.Status)
			}
			if state == "stream" {
				require.Equal(t, 3, log.Quota)
			} else {
				require.Zero(t, log.Quota)
			}
			require.Positive(t, log.FinishedAt)
		})
	}
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
