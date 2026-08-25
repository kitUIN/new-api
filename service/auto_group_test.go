package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestGetUserAutoGroupUsesPriorityOrderByDefault(t *testing.T) {
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalOrderType := setting.AutoGroupOrderType
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		setting.AutoGroupOrderType = originalOrderType
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"g1":"G1","g2":"G2"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"g1":2,"g2":1}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["g1","g2"]`))
	setting.UpdateAutoGroupOrderType(setting.AutoGroupOrderTypePriority)

	require.Equal(t, []string{"g1", "g2"}, GetUserAutoGroup(""))
}

func TestGetUserAutoGroupCanSortByEffectiveRatio(t *testing.T) {
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalOrderType := setting.AutoGroupOrderType
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		setting.AutoGroupOrderType = originalOrderType
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"g1":"G1","g2":"G2","g3":"G3"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"g1":2,"g2":1,"g3":3}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"g3":0.5}}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["g1","g2","g3"]`))
	setting.UpdateAutoGroupOrderType(setting.AutoGroupOrderTypeRatioAsc)

	require.Equal(t, []string{"g3", "g2", "g1"}, GetUserAutoGroup("vip"))
}

func TestGetUserAutoGroupExcludesCombinationGroups(t *testing.T) {
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"g1":"G1","combo":"Combo"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"g1":1,"combo":1}`))
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(`{"combo":{"gpt-5.6-sol":1}}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["combo","g1"]`))

	require.Equal(t, []string{"g1"}, GetUserAutoGroup(""))
}
