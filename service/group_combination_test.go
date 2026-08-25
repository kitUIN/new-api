package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveGroupCombinationChannel(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})

	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, db.Create(&model.Channel{
		Id: 7, Name: "sol", Key: "sk-sol", Status: common.ChannelStatusEnabled,
		Models: "gpt-5.6-sol", Group: "source",
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 8, Name: "disabled", Key: "sk-disabled", Status: common.ChannelStatusManuallyDisabled,
		Models: "gpt-5.6-luna", Group: "source",
	}).Error)
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"codex":{"gpt-5.6-sol":7,"gpt-5.6-luna":8}}`,
	))

	channel, enabled, err := ResolveGroupCombinationChannel("codex", "gpt-5.6-sol")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, 7, channel.Id)

	_, enabled, err = ResolveGroupCombinationChannel("codex", "missing")
	require.True(t, enabled)
	require.ErrorContains(t, err, "未配置模型")

	_, enabled, err = ResolveGroupCombinationChannel("codex", "gpt-5.6-luna")
	require.True(t, enabled)
	require.ErrorContains(t, err, "已停用")

	channel, enabled, err = ResolveGroupCombinationChannel("default", "gpt-5.6-sol")
	require.NoError(t, err)
	require.False(t, enabled)
	require.Nil(t, channel)
}
