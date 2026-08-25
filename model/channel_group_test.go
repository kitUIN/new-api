package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestHasEnabledChannelInGroup(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Name:   "enabled",
		Key:    "sk-enabled",
		Status: common.ChannelStatusEnabled,
		Group:  "default,vip",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id:     2,
		Name:   "disabled",
		Key:    "sk-disabled",
		Status: common.ChannelStatusAutoDisabled,
		Group:  "svip",
	}).Error)

	has, err := HasEnabledChannelInGroup("default")
	require.NoError(t, err)
	require.True(t, has)

	has, err = HasEnabledChannelInGroup("vip")
	require.NoError(t, err)
	require.True(t, has)

	has, err = HasEnabledChannelInGroup("svip")
	require.NoError(t, err)
	require.False(t, has)

	has, err = HasEnabledChannelInGroup("missing")
	require.NoError(t, err)
	require.False(t, has)
}

func TestGetEnabledChannelGroupSet(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Name:   "enabled",
		Key:    "sk-enabled",
		Status: common.ChannelStatusEnabled,
		Group:  "cheap,expensive",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id:     2,
		Name:   "disabled",
		Key:    "sk-disabled",
		Status: common.ChannelStatusManuallyDisabled,
		Group:  "disabled",
	}).Error)

	groups, err := GetEnabledChannelGroupSet()
	require.NoError(t, err)
	require.True(t, groups["cheap"])
	require.True(t, groups["expensive"])
	require.False(t, groups["disabled"])
}

func TestGetChannelGroupBindings(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Name:   "primary",
		Key:    "sk-primary",
		Status: common.ChannelStatusEnabled,
		Group:  " default, vip, vip ",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id:     2,
		Name:   "fallback",
		Key:    "sk-fallback",
		Status: common.ChannelStatusAutoDisabled,
		Group:  "vip",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id:     3,
		Name:   "unbound",
		Key:    "sk-unbound",
		Status: common.ChannelStatusEnabled,
		Group:  ",",
	}).Error)

	bindings, err := GetChannelGroupBindings()
	require.NoError(t, err)
	require.Equal(t, []GroupBoundChannel{
		{Id: 1, Name: "primary", Status: common.ChannelStatusEnabled, Models: []string{}},
	}, bindings["default"])
	require.Equal(t, []GroupBoundChannel{
		{Id: 1, Name: "primary", Status: common.ChannelStatusEnabled, Models: []string{}},
		{Id: 2, Name: "fallback", Status: common.ChannelStatusAutoDisabled, Models: []string{}},
	}, bindings["vip"])
	require.NotContains(t, bindings, "")
}

func TestCombinationGroupModelsAndAvailability(t *testing.T) {
	truncateTables(t)
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})

	require.NoError(t, DB.Create(&Channel{
		Id: 1, Name: "sol", Key: "sk-sol", Status: common.ChannelStatusEnabled,
		Models: "gpt-5.6-sol", Group: "source-sol",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id: 2, Name: "luna", Key: "sk-luna", Status: common.ChannelStatusEnabled,
		Models: "gpt-5.6-luna", Group: "source-luna",
	}).Error)
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"codex":{"gpt-5.6-sol":1,"gpt-5.6-luna":2}}`,
	))

	require.Equal(t, []string{"gpt-5.6-luna", "gpt-5.6-sol"}, GetGroupEnabledModels("codex"))
	require.True(t, HasAvailableChannelForGroupModel("codex", "gpt-5.6-sol"))
	require.True(t, IsChannelEnabledForGroupModel("codex", "gpt-5.6-luna", 2))
	require.False(t, IsChannelEnabledForGroupModel("codex", "gpt-5.6-luna", 1))

	groups, err := GetEnabledChannelGroupSet()
	require.NoError(t, err)
	require.True(t, groups["codex"])
}

func TestSupportsMappedModel(t *testing.T) {
	mapping := `{"alias":"vendor-alias","vendor-alias":"gpt-5.6-sol"}`
	channel := &Channel{Models: "alias", ModelMapping: &mapping}
	require.True(t, channel.SupportsMappedModel(JuiceTestModel))
	resolved, ok := channel.ResolveMappedModel(JuiceTestModel)
	require.True(t, ok)
	require.Equal(t, "alias", resolved)
	require.False(t, channel.SupportsMappedModel("gpt-5.4"))

	invalidMapping := `{`
	channel.ModelMapping = &invalidMapping
	require.False(t, channel.SupportsMappedModel(JuiceTestModel))

	mappedAway := `{"gpt-5.6-sol":"gpt-5.4"}`
	channel.Models = JuiceTestModel
	channel.ModelMapping = &mappedAway
	require.False(t, channel.SupportsMappedModel(JuiceTestModel))
}

func TestGetGroupJuiceStatsUsesMinimumArbitraryPrecisionValue(t *testing.T) {
	truncateTables(t)
	mapping := `{"alias":"gpt-5.6-sol"}`
	require.NoError(t, DB.Create(&Channel{
		Id: 1, Name: "large", Key: "sk-1", Status: common.ChannelStatusEnabled,
		Models: JuiceTestModel, Group: "default,vip",
		Juice: "999999999999999999999999999999999999", JuiceUpdatedTime: 200,
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id: 2, Name: "mapped", Key: "sk-2", Status: common.ChannelStatusEnabled,
		Models: "alias", ModelMapping: &mapping, Group: "default",
		Juice: "9", JuiceUpdatedTime: 300,
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id: 3, Name: "disabled", Key: "sk-3", Status: common.ChannelStatusManuallyDisabled,
		Models: JuiceTestModel, Group: "default", Juice: "1", JuiceUpdatedTime: 100,
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id: 4, Name: "invalid", Key: "sk-4", Status: common.ChannelStatusEnabled,
		Models: JuiceTestModel, Group: "default", Juice: "+1", JuiceUpdatedTime: 50,
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id: 5, Name: "decimal", Key: "sk-5", Status: common.ChannelStatusEnabled,
		Models: JuiceTestModel, Group: "decimal", Juice: "0.75", JuiceUpdatedTime: 400,
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id: 6, Name: "smaller-decimal", Key: "sk-6", Status: common.ChannelStatusEnabled,
		Models: JuiceTestModel, Group: "decimal", Juice: "0.5", JuiceUpdatedTime: 500,
	}).Error)

	stats, err := GetGroupJuiceStats()
	require.NoError(t, err)
	require.Equal(t, "9", stats["default"].Juice)
	require.EqualValues(t, 200, stats["default"].UpdatedTime)
	require.Equal(t, "999999999999999999999999999999999999", stats["vip"].Juice)
	require.EqualValues(t, 200, stats["vip"].UpdatedTime)
	require.Equal(t, "0.5", stats["decimal"].Juice)
	require.EqualValues(t, 400, stats["decimal"].UpdatedTime)
}

func TestUpdateJuiceTestPreservesLastValueOnFailure(t *testing.T) {
	truncateTables(t)
	channel := &Channel{
		Id: 1, Name: "juice", Key: "sk", Status: common.ChannelStatusEnabled,
		Models: JuiceTestModel, Juice: "42", JuiceUpdatedTime: 100,
	}
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.UpdateJuiceTest("", "upstream failed"))

	var stored Channel
	require.NoError(t, DB.First(&stored, 1).Error)
	require.Equal(t, "42", stored.Juice)
	require.EqualValues(t, 100, stored.JuiceUpdatedTime)
	require.Equal(t, "upstream failed", stored.JuiceTestError)
	require.Positive(t, stored.JuiceTestTime)
}

func TestUpdateJuiceTestRecordsGroupMinimumChanges(t *testing.T) {
	truncateTables(t)
	primary := &Channel{
		Id: 1, Name: "primary", Key: "sk-1", Status: common.ChannelStatusEnabled,
		Models: JuiceTestModel, Group: "default", Juice: "10", JuiceUpdatedTime: 100,
	}
	fallback := &Channel{
		Id: 2, Name: "fallback", Key: "sk-2", Status: common.ChannelStatusEnabled,
		Models: JuiceTestModel, Group: "default", Juice: "20", JuiceUpdatedTime: 100,
	}
	require.NoError(t, DB.Create(primary).Error)
	require.NoError(t, DB.Create(fallback).Error)

	require.NoError(t, primary.UpdateJuiceTestWithSource("30", "", GroupJuiceHistorySourceScheduled))

	var histories []GroupJuiceHistory
	require.NoError(t, DB.Order("id ASC").Find(&histories).Error)
	require.Len(t, histories, 1)
	require.Equal(t, "default", histories[0].Group)
	require.Equal(t, "10", histories[0].OldJuice)
	require.Equal(t, "20", histories[0].NewJuice)
	require.Equal(t, primary.Id, histories[0].ChannelId)
	require.Equal(t, GroupJuiceHistorySourceScheduled, histories[0].Source)

	require.NoError(t, fallback.UpdateJuiceTestWithSource("20.0", "", GroupJuiceHistorySourceManual))
	require.NoError(t, DB.Order("id ASC").Find(&histories).Error)
	require.Len(t, histories, 1, "numerically equal Juice values must not create history")
}

func TestUpdateJuiceTestSkipsInitialGroupJuiceHistory(t *testing.T) {
	truncateTables(t)
	channel := &Channel{
		Id: 1, Name: "initial", Key: "sk", Status: common.ChannelStatusEnabled,
		Models: JuiceTestModel, Group: "default",
	}
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.UpdateJuiceTestWithSource("128", "", GroupJuiceHistorySourceScheduled))

	var count int64
	require.NoError(t, DB.Model(&GroupJuiceHistory{}).Count(&count).Error)
	require.Zero(t, count)
}
