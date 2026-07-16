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
