package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
		{Id: 1, Name: "primary", Status: common.ChannelStatusEnabled},
	}, bindings["default"])
	require.Equal(t, []GroupBoundChannel{
		{Id: 1, Name: "primary", Status: common.ChannelStatusEnabled},
		{Id: 2, Name: "fallback", Status: common.ChannelStatusAutoDisabled},
	}, bindings["vip"])
	require.NotContains(t, bindings, "")
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

	stats, err := GetGroupJuiceStats()
	require.NoError(t, err)
	require.Equal(t, "9", stats["default"].Juice)
	require.EqualValues(t, 200, stats["default"].UpdatedTime)
	require.Equal(t, "999999999999999999999999999999999999", stats["vip"].Juice)
	require.EqualValues(t, 200, stats["vip"].UpdatedTime)
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
