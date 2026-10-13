package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestTaskTimedRatioSnapshotSurvivesExpiry(t *testing.T) {
	truncate(t)
	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4":2}`))
	const userID = 9241
	seedUser(t, userID, 10000)
	task := makeTask(userID, 0, 5000, 0, BillingSourceWallet, 0)
	bc := task.PrivateData.BillingContext
	bc.OriginModelName = "gpt-4"
	bc.GroupRatio = 0.25
	bc.TimedRatio = &types.GroupTimedRatioSnapshot{
		OriginalRatio: 2, Ratio: 0.25, Start: time.Now().Add(-2 * time.Hour).Unix(), End: time.Now().Add(-time.Hour).Unix(), Remark: "expired promotion",
	}
	// Round-trip through the same JSON representation used for persisted tasks.
	encoded, err := common.Marshal(task.PrivateData)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(encoded, &task.PrivateData))
	modelRatio, exists, _ := ratio_setting.GetModelRatio("gpt-4")
	require.True(t, exists)
	expected := int(100 * modelRatio * 0.25)
	RecalculateTaskQuotaByTokens(context.Background(), task, 100)
	require.Equal(t, expected, task.Quota)
	require.Equal(t, 10000+5000-expected, getUserQuota(t, userID))
	other := taskBillingOther(task)
	require.Equal(t, 0.25, other["group_ratio"])
	require.Equal(t, bc.TimedRatio, other["timed_group_ratio"])
	var record model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ?", userID).First(&record).Error)
	var logged struct {
		TimedRatio *types.GroupTimedRatioSnapshot `json:"timed_group_ratio"`
	}
	require.NoError(t, common.UnmarshalJsonStr(record.Other, &logged))
	require.Equal(t, bc.TimedRatio, logged.TimedRatio)
}
