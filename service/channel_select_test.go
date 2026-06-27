package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRetryParamReleasesOldestExcludedChannelAfterAllCandidatesFailed(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Ability{}, &model.Channel{}))

	priority := int64(10)
	weight := uint(1)
	channels := []*model.Channel{
		{
			Id:       1,
			Name:     "primary",
			Key:      "sk-primary",
			Status:   common.ChannelStatusEnabled,
			Group:    "default",
			Models:   "gpt-test",
			Priority: &priority,
			Weight:   &weight,
		},
		{
			Id:       2,
			Name:     "secondary",
			Key:      "sk-secondary",
			Status:   common.ChannelStatusEnabled,
			Group:    "default",
			Models:   "gpt-test",
			Priority: &priority,
			Weight:   &weight,
		},
	}
	for _, channel := range channels {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}

	param := &RetryParam{
		TokenGroup:         "default",
		ModelName:          "gpt-test",
		ExcludedChannelIDs: []int{1, 2},
	}

	channel, err := param.nextChannelWithRetryExclusions("default", 0)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1, channel.Id)
	require.Equal(t, []int{2}, param.ExcludedChannelIDs)
}
