package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogQuotaDataPersistsTokenBreakdown(t *testing.T) {
	truncateTables(t)
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()

	createdAt := int64(1710000123)
	LogQuotaData(1, "alice", "gpt-test", "default", 200, createdAt, 170, 100, 40, 20, 10)
	LogQuotaData(1, "alice", "gpt-test", "default", 300, createdAt+10, 255, 150, 60, 30, 15)

	SaveQuotaDataCache()

	var quotaData QuotaData
	require.NoError(t, DB.Table("quota_data").Where("user_id = ? AND model_name = ?", 1, "gpt-test").First(&quotaData).Error)
	require.Equal(t, "default", quotaData.Group)
	require.Equal(t, 2, quotaData.Count)
	require.Equal(t, 500, quotaData.Quota)
	require.Equal(t, 425, quotaData.TokenUsed)
	require.Equal(t, 250, quotaData.PromptTokens)
	require.Equal(t, 100, quotaData.CompletionTokens)
	require.Equal(t, 50, quotaData.CacheReadTokens)
	require.Equal(t, 25, quotaData.CacheWriteTokens)
}

func TestSaveQuotaDataCacheIncrementsExistingTokenBreakdown(t *testing.T) {
	truncateTables(t)
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()

	createdAt := int64(1710000000)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:           2,
		Username:         "bob",
		ModelName:        "gpt-test",
		Group:            "vip",
		CreatedAt:        createdAt,
		Count:            1,
		Quota:            100,
		TokenUsed:        80,
		PromptTokens:     50,
		CompletionTokens: 20,
		CacheReadTokens:  5,
		CacheWriteTokens: 5,
	}).Error)

	LogQuotaData(2, "bob", "gpt-test", "vip", 150, createdAt+1, 120, 70, 30, 10, 10)
	SaveQuotaDataCache()

	var quotaData QuotaData
	require.NoError(t, DB.Table("quota_data").Where("user_id = ? AND model_name = ?", 2, "gpt-test").First(&quotaData).Error)
	require.Equal(t, "vip", quotaData.Group)
	require.Equal(t, 2, quotaData.Count)
	require.Equal(t, 250, quotaData.Quota)
	require.Equal(t, 200, quotaData.TokenUsed)
	require.Equal(t, 120, quotaData.PromptTokens)
	require.Equal(t, 50, quotaData.CompletionTokens)
	require.Equal(t, 15, quotaData.CacheReadTokens)
	require.Equal(t, 15, quotaData.CacheWriteTokens)
}

func TestLogQuotaDataSeparatesGroups(t *testing.T) {
	truncateTables(t)
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()

	createdAt := int64(1710000123)
	LogQuotaData(4, "dave", "gpt-test", "default", 100, createdAt, 70, 40, 20, 5, 5)
	LogQuotaData(4, "dave", "gpt-test", "vip", 200, createdAt+20, 140, 80, 40, 10, 10)

	SaveQuotaDataCache()

	var rows []QuotaData
	require.NoError(t, DB.Table("quota_data").Where("user_id = ? AND model_name = ?", 4, "gpt-test").Order(commonGroupCol).Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, "default", rows[0].Group)
	require.Equal(t, 100, rows[0].Quota)
	require.Equal(t, 70, rows[0].TokenUsed)
	require.Equal(t, "vip", rows[1].Group)
	require.Equal(t, 200, rows[1].Quota)
	require.Equal(t, 140, rows[1].TokenUsed)
}

func TestGetQuotaDataGroupByGroupModel(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&QuotaData{
		UserID:           5,
		Username:         "erin",
		ModelName:        "gpt-a",
		Group:            "default",
		CreatedAt:        1710000000,
		Count:            1,
		Quota:            100,
		TokenUsed:        80,
		PromptTokens:     50,
		CompletionTokens: 20,
		CacheReadTokens:  5,
		CacheWriteTokens: 5,
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:           6,
		Username:         "frank",
		ModelName:        "gpt-a",
		Group:            "default",
		CreatedAt:        1710003600,
		Count:            2,
		Quota:            300,
		TokenUsed:        240,
		PromptTokens:     150,
		CompletionTokens: 60,
		CacheReadTokens:  20,
		CacheWriteTokens: 10,
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:           7,
		Username:         "gina",
		ModelName:        "gpt-b",
		Group:            "vip",
		CreatedAt:        1710000000,
		Count:            1,
		Quota:            500,
		TokenUsed:        400,
		PromptTokens:     250,
		CompletionTokens: 100,
		CacheReadTokens:  30,
		CacheWriteTokens: 20,
	}).Error)

	rows, err := GetQuotaDataGroupByGroupModel(1709990000, 1710010000)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byKey := make(map[string]*QuotaData)
	for _, row := range rows {
		byKey[row.Group+"|"+row.ModelName] = row
	}

	defaultModel := byKey["default|gpt-a"]
	require.NotNil(t, defaultModel)
	require.Equal(t, 3, defaultModel.Count)
	require.Equal(t, 400, defaultModel.Quota)
	require.Equal(t, 320, defaultModel.TokenUsed)
	require.Equal(t, 200, defaultModel.PromptTokens)
	require.Equal(t, 80, defaultModel.CompletionTokens)
	require.Equal(t, 25, defaultModel.CacheReadTokens)
	require.Equal(t, 15, defaultModel.CacheWriteTokens)

	vipModel := byKey["vip|gpt-b"]
	require.NotNil(t, vipModel)
	require.Equal(t, 1, vipModel.Count)
	require.Equal(t, 500, vipModel.Quota)
	require.Equal(t, 400, vipModel.TokenUsed)
}

func TestGetQuotaDataGroupByUserGroupModel(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&QuotaData{
		UserID:           8,
		Username:         "henry",
		ModelName:        "gpt-a",
		Group:            "default",
		CreatedAt:        1710000000,
		Count:            1,
		Quota:            100,
		TokenUsed:        80,
		PromptTokens:     50,
		CompletionTokens: 20,
		CacheReadTokens:  5,
		CacheWriteTokens: 5,
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:           8,
		Username:         "henry",
		ModelName:        "gpt-a",
		Group:            "default",
		CreatedAt:        1710003600,
		Count:            2,
		Quota:            300,
		TokenUsed:        240,
		PromptTokens:     150,
		CompletionTokens: 60,
		CacheReadTokens:  20,
		CacheWriteTokens: 10,
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:           9,
		Username:         "irene",
		ModelName:        "gpt-a",
		Group:            "default",
		CreatedAt:        1710000000,
		Count:            10,
		Quota:            900,
		TokenUsed:        700,
		PromptTokens:     400,
		CompletionTokens: 200,
		CacheReadTokens:  50,
		CacheWriteTokens: 50,
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:           8,
		Username:         "henry",
		ModelName:        "gpt-b",
		Group:            "vip",
		CreatedAt:        1710000000,
		Count:            1,
		Quota:            500,
		TokenUsed:        400,
		PromptTokens:     250,
		CompletionTokens: 100,
		CacheReadTokens:  30,
		CacheWriteTokens: 20,
	}).Error)

	rows, err := GetQuotaDataGroupByUserGroupModel(8, 1709990000, 1710010000)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byKey := make(map[string]*QuotaData)
	for _, row := range rows {
		byKey[row.Group+"|"+row.ModelName] = row
	}

	defaultModel := byKey["default|gpt-a"]
	require.NotNil(t, defaultModel)
	require.Equal(t, 3, defaultModel.Count)
	require.Equal(t, 400, defaultModel.Quota)
	require.Equal(t, 320, defaultModel.TokenUsed)
	require.Equal(t, 200, defaultModel.PromptTokens)
	require.Equal(t, 80, defaultModel.CompletionTokens)
	require.Equal(t, 25, defaultModel.CacheReadTokens)
	require.Equal(t, 15, defaultModel.CacheWriteTokens)

	vipModel := byKey["vip|gpt-b"]
	require.NotNil(t, vipModel)
	require.Equal(t, 1, vipModel.Count)
	require.Equal(t, 500, vipModel.Quota)
	require.Equal(t, 400, vipModel.TokenUsed)
}

func TestGetQuotaDataGroupByGroupUser(t *testing.T) {
	truncateTables(t)

	rows := []*QuotaData{
		{
			UserID:           10,
			Username:         "alice",
			ModelName:        "gpt-a",
			Group:            "default",
			CreatedAt:        1710000000,
			Count:            1,
			Quota:            100,
			TokenUsed:        80,
			PromptTokens:     50,
			CompletionTokens: 20,
			CacheReadTokens:  5,
			CacheWriteTokens: 5,
		},
		{
			UserID:           10,
			Username:         "alice",
			ModelName:        "gpt-b",
			Group:            "default",
			CreatedAt:        1710003600,
			Count:            2,
			Quota:            300,
			TokenUsed:        240,
			PromptTokens:     150,
			CompletionTokens: 60,
			CacheReadTokens:  20,
			CacheWriteTokens: 10,
		},
		{
			UserID:           11,
			Username:         "bob",
			ModelName:        "gpt-a",
			Group:            "default",
			CreatedAt:        1710000000,
			Count:            3,
			Quota:            500,
			TokenUsed:        400,
			PromptTokens:     250,
			CompletionTokens: 100,
			CacheReadTokens:  30,
			CacheWriteTokens: 20,
		},
		{
			UserID:           10,
			Username:         "alice",
			ModelName:        "gpt-c",
			Group:            "vip",
			CreatedAt:        1710000000,
			Count:            1,
			Quota:            200,
			TokenUsed:        160,
			PromptTokens:     100,
			CompletionTokens: 40,
			CacheReadTokens:  10,
			CacheWriteTokens: 10,
		},
	}
	require.NoError(t, DB.Create(rows).Error)

	allRows, err := GetQuotaDataGroupByGroupUser(0, 1709990000, 1710010000)
	require.NoError(t, err)
	require.Len(t, allRows, 3)

	byKey := make(map[string]*QuotaData)
	for _, row := range allRows {
		byKey[row.Group+"|"+row.Username] = row
	}

	aliceDefault := byKey["default|alice"]
	require.NotNil(t, aliceDefault)
	require.Equal(t, 10, aliceDefault.UserID)
	require.Equal(t, 3, aliceDefault.Count)
	require.Equal(t, 400, aliceDefault.Quota)
	require.Equal(t, 320, aliceDefault.TokenUsed)
	require.Equal(t, 200, aliceDefault.PromptTokens)
	require.Equal(t, 80, aliceDefault.CompletionTokens)
	require.Equal(t, 25, aliceDefault.CacheReadTokens)
	require.Equal(t, 15, aliceDefault.CacheWriteTokens)

	filteredRows, err := GetQuotaDataGroupByGroupUser(10, 1709990000, 1710010000)
	require.NoError(t, err)
	require.Len(t, filteredRows, 2)
	for _, row := range filteredRows {
		require.Equal(t, 10, row.UserID)
		require.Equal(t, "alice", row.Username)
	}
}

func TestGetQuotaDataUsers(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create([]*QuotaData{
		{
			UserID:    12,
			Username:  "carol",
			ModelName: "gpt-a",
			Group:     "default",
			CreatedAt: 1710000000,
			Quota:     100,
		},
		{
			UserID:    12,
			Username:  "carol",
			ModelName: "gpt-b",
			Group:     "vip",
			CreatedAt: 1710003600,
			Quota:     300,
		},
		{
			UserID:    13,
			Username:  "dave",
			ModelName: "gpt-a",
			Group:     "default",
			CreatedAt: 1710000000,
			Quota:     500,
		},
		{
			UserID:    14,
			Username:  "outside",
			ModelName: "gpt-a",
			Group:     "default",
			CreatedAt: 1711000000,
			Quota:     900,
		},
	}).Error)

	users, err := GetQuotaDataUsers(1709990000, 1710010000)
	require.NoError(t, err)
	require.Len(t, users, 2)

	byID := make(map[int]*QuotaDataUser)
	for _, user := range users {
		byID[user.UserID] = user
	}
	require.Equal(t, "carol", byID[12].Username)
	require.Equal(t, 400, byID[12].Quota)
	require.Equal(t, "dave", byID[13].Username)
	require.Equal(t, 500, byID[13].Quota)
}

func TestGetAllQuotaDatesReturnsZeroBreakdownForLegacyRows(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&QuotaData{
		UserID:    3,
		Username:  "carol",
		ModelName: "legacy-model",
		CreatedAt: 1710000000,
		Count:     1,
		Quota:     100,
		TokenUsed: 80,
	}).Error)

	rows, err := GetAllQuotaDates(1709990000, 1710010000, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "legacy-model", rows[0].ModelName)
	require.Equal(t, 80, rows[0].TokenUsed)
	require.Zero(t, rows[0].PromptTokens)
	require.Zero(t, rows[0].CompletionTokens)
	require.Zero(t, rows[0].CacheReadTokens)
	require.Zero(t, rows[0].CacheWriteTokens)
}
