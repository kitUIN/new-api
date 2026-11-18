package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRefreshLogsScopesIDsAndUpdatesPendingRows(t *testing.T) {
	truncateTables(t)
	c, info, id := startTestUsageLog(t, "pinned-request")
	other := &Log{UserId: 99, Type: LogTypeConsume}
	require.NoError(t, LOG_DB.Create(other).Error)
	RecordConsumeLog(c, 42, RecordConsumeLogParams{ModelName: "test-model", Quota: 30, CompletionTokens: 12})
	FinishRequestLog(c, info, nil)
	logs, err := GetLogsByIDs(42, []int{other.Id, id, id})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, id, logs[0].Id)
	require.Equal(t, "completed", logs[0].Status)
	require.Equal(t, 30, logs[0].Quota)
	require.Equal(t, 12, logs[0].CompletionTokens)
	logs, err = GetLogsByIDs(123, []int{id, other.Id})
	require.NoError(t, err)
	require.Empty(t, logs)
	logs, err = GetLogsByIDs(0, []int{id, other.Id})
	require.NoError(t, err)
	require.Len(t, logs, 2)
}

func TestRefreshLogsPreservesAdminChannelAndUserMetadata(t *testing.T) {
	truncateTables(t)
	oldMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = oldMemoryCache })
	require.NoError(t, DB.Create(&User{Id: 42, Username: "refresh-user", QQId: "12345"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 8, Name: "Refresh channel"}).Error)
	log := &Log{UserId: 42, ChannelId: 8, Type: LogTypeConsume}
	require.NoError(t, LOG_DB.Create(log).Error)
	logs, err := GetLogsByIDs(0, []int{log.Id})
	require.NoError(t, err)
	require.Equal(t, "Refresh channel", logs[0].ChannelName)
	require.Equal(t, "12345", logs[0].QQId)
}

func TestRefreshLogsRejectsInvalidBatches(t *testing.T) {
	for _, ids := range [][]int{nil, {}, {0}, {-1}, make([]int, MaxLogRefreshIDs+1)} {
		_, err := GetLogsByIDs(42, ids)
		require.Error(t, err)
	}
	_, err := GetLogsByIDs(-1, []int{1})
	require.Error(t, err)
}
