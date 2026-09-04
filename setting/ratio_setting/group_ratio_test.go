package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	settingconfig "github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestApplyGroupRatioBindingLocksToJSONString(t *testing.T) {
	originalRatio := GroupRatio2JSONString()
	originalBindings := UpstreamGroupRatioBindings2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioByJSONString(originalRatio))
		require.NoError(t, UpdateUpstreamGroupRatioBindingsByJSONString(originalBindings))
	})

	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":1,"vip":0.8}`))
	require.NoError(t, UpdateUpstreamGroupRatioBindingsByJSONString(`{
		"vip": {
			"source_type": "channel",
			"source_id": 1,
			"upstream_group": "vip",
			"offset": 0.01
		}
	}`))

	locked, err := ApplyGroupRatioBindingLocksToJSONString(`{"default":2,"vip":9,"new":3}`)
	require.NoError(t, err)

	var got map[string]float64
	require.NoError(t, common.Unmarshal([]byte(locked), &got))
	require.Equal(t, 2.0, got["default"])
	require.Equal(t, 0.8, got["vip"])
	require.Equal(t, 3.0, got["new"])
}

func TestCheckUpstreamGroupRatioBindings(t *testing.T) {
	require.NoError(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "provider",
			"source_id": 2,
			"upstream_group": "default",
			"offset": -0.05
		}
	}`))
	require.NoError(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "provider",
			"source_id": 2,
			"upstream_group": "default",
			"offset": "(x + 0.3) / 10 + 0.4"
		}
	}`))
	require.NoError(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "provider",
			"source_id": 2,
			"upstream_group": "default",
			"offset_expr": "(x + 0.3) / 10 + 0.4"
		}
	}`))

	require.Error(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "invalid",
			"source_id": 2,
			"upstream_group": "default"
		}
	}`))
	require.Error(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "channel",
			"source_id": 0,
			"upstream_group": "default"
		}
	}`))
	require.Error(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "channel",
			"source_id": 1,
			"upstream_group": ""
		}
	}`))
	require.Error(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "channel",
			"source_id": 1,
			"upstream_group": "default",
			"offset": "x + "
		}
	}`))
}

func TestCalculateUpstreamGroupBoundRatio(t *testing.T) {
	ratio, err := CalculateUpstreamGroupBoundRatio(1.2, UpstreamGroupRatioBinding{Offset: 0.01})
	require.NoError(t, err)
	require.Equal(t, 1.21, ratio)

	ratio, err = CalculateUpstreamGroupBoundRatio(1.2, UpstreamGroupRatioBinding{OffsetExpression: "(x + 0.3) / 10 + 0.4"})
	require.NoError(t, err)
	require.InDelta(t, 0.55, ratio, 1e-9)

	ratio, err = CalculateUpstreamGroupBoundRatio(0.8, UpstreamGroupRatioBinding{OffsetExpression: "x - 2"})
	require.NoError(t, err)
	require.Equal(t, 0.0, ratio)

	_, err = CalculateUpstreamGroupBoundRatio(1.2, UpstreamGroupRatioBinding{OffsetExpression: "x + "})
	require.Error(t, err)
}

func TestCheckGroupRatioRejectsNegative(t *testing.T) {
	require.Error(t, CheckGroupRatio(`{"default":-0.1}`))
	require.NoError(t, CheckGroupRatio(`{"default":0}`))
}

func TestCheckGroupTypes(t *testing.T) {
	require.NoError(t, CheckGroupTypes(`{"default":"billing","vip":"user"}`))
	require.NoError(t, CheckGroupTypes(`{}`))
	require.Error(t, CheckGroupTypes(`{"default":"invalid"}`))
	require.Error(t, CheckGroupTypes(`{"default":1}`))
}

func TestUpdateGroupTypesByJSONString(t *testing.T) {
	originalGroupTypes := GroupTypes2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupTypesByJSONString(originalGroupTypes))
	})

	require.NoError(t, UpdateGroupTypesByJSONString(`{"vip":"user"}`))
	require.Equal(t, GroupTypeUser, GetGroupType("vip"))
	require.Equal(t, GroupTypeBilling, GetGroupType("missing"))

	require.Error(t, UpdateGroupTypesByJSONString(`{"vip":"invalid"}`))
	require.Equal(t, GroupTypeUser, GetGroupType("vip"))
}

func TestGroupCombinations(t *testing.T) {
	original := GroupCombinations2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupCombinationsByJSONString(original))
	})

	value := `{"codex":[{"group":"cheap","models":["luna"]},{"group":"premium","models":["luna","sol"]}]}`
	require.NoError(t, CheckGroupCombinations(value))
	require.NoError(t, UpdateGroupCombinationsByJSONString(value))
	require.True(t, IsGroupCombination("codex"))

	members, configured := GetGroupCombinationMembers("codex")
	require.True(t, configured)
	require.Equal(t, []GroupCombinationMember{
		{Group: "cheap", Models: []string{"luna"}},
		{Group: "premium", Models: []string{"luna", "sol"}},
	}, members)
	require.True(t, GroupCombinationMemberSupportsModel(members[0], "luna"))
	require.False(t, GroupCombinationMemberSupportsModel(members[0], "sol"))
	require.JSONEq(t, value, GroupCombinations2JSONString())

	_, configured = GetGroupCombinationMembers("missing")
	require.False(t, configured)

	require.Error(t, CheckGroupCombinations(`{"codex":[]}`))
	require.Error(t, CheckGroupCombinations(`{"codex":[{"group":"cheap","models":["luna"]}]}`))
	require.Error(t, CheckGroupCombinations(`{"codex":[{"group":"cheap","models":["luna"]},{"group":"","models":["sol"]}]}`))
	require.Error(t, CheckGroupCombinations(`{"codex":[{"group":"cheap","models":["luna"]},{"group":"cheap","models":["sol"]}]}`))
	require.NoError(t, CheckGroupCombinations(`{"codex":[{"group":"cheap","models":["luna"]},{"group":"codex","models":["luna","sol"]}]}`))
	require.Error(t, CheckGroupCombinations(`{"codex":[{"group":"cheap","models":["luna"]},{"group":"nested","models":["sol"]}],"nested":[{"group":"a","models":["luna"]},{"group":"b","models":["sol"]}]}`))
	require.Error(t, CheckGroupCombinations(`{"a":[{"group":"b","models":["luna"]},{"group":"a","models":["luna"]}],"b":[{"group":"a","models":["luna"]},{"group":"b","models":["luna"]}]}`))
	require.Error(t, CheckGroupCombinations(`{" auto":[{"group":"cheap","models":["luna"]},{"group":"premium","models":["sol"]}]}`))
	require.Error(t, CheckGroupCombinations(`{"codex":[{"group":"cheap","models":[]},{"group":"premium","models":["sol"]}]}`))
	require.Error(t, CheckGroupCombinations(`{"codex":[{"group":"cheap","models":["luna","luna"]},{"group":"premium","models":["sol"]}]}`))
	require.Error(t, CheckGroupCombinations(`{"codex":[{"group":"cheap","models":[" luna"]},{"group":"premium","models":["sol"]}]}`))
	require.Error(t, CheckGroupCombinations(`{"codex":["cheap","premium"]}`))
}

func TestLegacyGroupCombinationsRemainRoutableAndAtomic(t *testing.T) {
	original := GroupCombinations2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupCombinationsByJSONString(original))
	})

	legacy := `{"codex":{"gpt-5.6-sol":7}}`
	require.NoError(t, CheckGroupCombinations(legacy))
	require.NoError(t, UpdateGroupCombinationsByJSONString(legacy))
	require.True(t, IsGroupCombination("codex"))
	require.True(t, IsLegacyGroupCombination("codex"))
	require.JSONEq(t, legacy, GroupCombinations2JSONString())

	channelID, configured, routed := GetGroupCombinationChannelID("codex", "gpt-5.6-sol")
	require.True(t, configured)
	require.True(t, routed)
	require.Equal(t, 7, channelID)
	_, configured = GetGroupCombinationMembers("codex")
	require.False(t, configured)

	invalid := `{"codex":[{"group":"only","models":["gpt-5.6-sol"]}]}`
	require.Error(t, settingconfig.UpdateConfigFromMap(GetGroupRatioSetting(), map[string]string{
		"group_combinations": invalid,
	}))
	require.JSONEq(t, legacy, GroupCombinations2JSONString())
}

func TestCompareGroupRatioChanges(t *testing.T) {
	changes := CompareGroupRatioChanges(
		map[string]float64{
			"default": 1,
			"old":     0.5,
			"vip":     0.8,
		},
		map[string]float64{
			"default": 1,
			"new":     1.2,
			"vip":     0.9,
		},
	)

	require.Equal(t, []GroupRatioChange{
		{Type: GroupRatioChangeAdded, Group: "new", NewRatio: 1.2},
		{Type: GroupRatioChangeDeleted, Group: "old", OldRatio: 0.5},
		{Type: GroupRatioChangeUpdated, Group: "vip", OldRatio: 0.8, NewRatio: 0.9},
	}, changes)
	require.Equal(t, []string{
		"+ new: 倍率 1.2",
		"- old: 原倍率 0.5",
		"* vip: 0.8 -> 0.9",
	}, FormatGroupRatioChangeLines(changes))
}

func TestCompareGroupRatioChangesIgnoresEqualMaps(t *testing.T) {
	changes := CompareGroupRatioChanges(
		map[string]float64{
			"default": 1,
			"vip":     0.8,
		},
		map[string]float64{
			"vip":     0.8,
			"default": 1,
		},
	)

	require.Empty(t, changes)
	require.Empty(t, FormatGroupRatioChangeMessage(changes))
}

func TestFilterGroupRatioChangesByEnabledGroups(t *testing.T) {
	changes := []GroupRatioChange{
		{Type: GroupRatioChangeUpdated, Group: "default", OldRatio: 1, NewRatio: 0.9},
		{Type: GroupRatioChangeUpdated, Group: "disabled", OldRatio: 1, NewRatio: 0.8},
		{Type: GroupRatioChangeDeleted, Group: "old", OldRatio: 0.5},
	}

	filtered := FilterGroupRatioChangesByEnabledGroups(
		changes,
		map[string]string{
			"default": "默认分组",
			"old":     "旧分组",
		},
	)

	require.Equal(t, []GroupRatioChange{
		{Type: GroupRatioChangeUpdated, Group: "default", OldRatio: 1, NewRatio: 0.9},
		{Type: GroupRatioChangeDeleted, Group: "old", OldRatio: 0.5},
	}, filtered)
	require.Equal(t, []string{
		"* default: 1 -> 0.9",
		"- old: 原倍率 0.5",
	}, FormatGroupRatioChangeLines(filtered))
}

func TestAppendGroupCombinationRatioChanges(t *testing.T) {
	changes := []GroupRatioChange{
		{Type: GroupRatioChangeUpdated, Group: "cheap", OldRatio: 0.5, NewRatio: 0.6},
		{Type: GroupRatioChangeUpdated, Group: "unrelated", OldRatio: 1, NewRatio: 1.1},
	}

	withCombinations := AppendGroupCombinationRatioChanges(
		changes,
		map[string][]GroupCombinationMember{
			"combo-b": {
				{Group: "cheap", Models: []string{"model-b"}},
				{Group: "premium", Models: []string{"model-b"}},
			},
			"combo-a": {
				{Group: "cheap", Models: []string{"model-a"}},
				{Group: "combo-a", Models: []string{"model-a"}},
			},
		},
	)

	require.Equal(t, []GroupRatioChange{
		{Type: GroupRatioChangeUpdated, Group: "cheap", OldRatio: 0.5, NewRatio: 0.6},
		{Type: GroupRatioChangeUpdated, Group: "unrelated", OldRatio: 1, NewRatio: 1.1},
		{Type: GroupRatioChangeUpdated, Group: "combo-a", CombinationMember: "cheap", OldRatio: 0.5, NewRatio: 0.6},
		{Type: GroupRatioChangeUpdated, Group: "combo-b", CombinationMember: "cheap", OldRatio: 0.5, NewRatio: 0.6},
	}, withCombinations)
	require.Equal(t, []string{
		"* cheap: 0.5 -> 0.6",
		"* unrelated: 1 -> 1.1",
		"* combo-a: 组合成员 cheap 倍率 0.5 -> 0.6",
		"* combo-b: 组合成员 cheap 倍率 0.5 -> 0.6",
	}, FormatGroupRatioChangeLines(withCombinations))
}

func TestCombinationRatioChangeSurvivesEnabledGroupFilter(t *testing.T) {
	changes := AppendGroupCombinationRatioChanges(
		[]GroupRatioChange{
			{Type: GroupRatioChangeUpdated, Group: "internal", OldRatio: 0.4, NewRatio: 0.5},
		},
		map[string][]GroupCombinationMember{
			"public-combo": {
				{Group: "internal", Models: []string{"model"}},
				{Group: "fallback", Models: []string{"model"}},
			},
		},
	)

	filtered := FilterGroupRatioChangesByEnabledGroups(
		changes,
		map[string]string{"public-combo": "公开组合分组"},
	)

	require.Equal(t, []GroupRatioChange{
		{
			Type:              GroupRatioChangeUpdated,
			Group:             "public-combo",
			CombinationMember: "internal",
			OldRatio:          0.4,
			NewRatio:          0.5,
		},
	}, filtered)
}

func TestFormatAddedAndDeletedCombinationMemberRatioChanges(t *testing.T) {
	changes := []GroupRatioChange{
		{Type: GroupRatioChangeAdded, Group: "combo", CombinationMember: "new", NewRatio: 0.7},
		{Type: GroupRatioChangeDeleted, Group: "combo", CombinationMember: "old", OldRatio: 0.8},
	}

	require.Equal(t, []string{
		"* combo: 组合成员 new 新增倍率 0.7",
		"* combo: 组合成员 old 移除倍率，原倍率 0.8",
	}, FormatGroupRatioChangeLines(changes))
}

func TestCompareUpstreamGroupRatioBindingChanges(t *testing.T) {
	changes := CompareUpstreamGroupRatioBindingChanges(
		map[string]UpstreamGroupRatioBinding{
			"old": {
				SourceType:    UpstreamGroupRatioBindingSourceProvider,
				SourceID:      2,
				UpstreamGroup: "old-upstream",
			},
			"vip": {
				SourceType:    UpstreamGroupRatioBindingSourceChannel,
				SourceID:      1,
				UpstreamGroup: "vip",
			},
		},
		map[string]UpstreamGroupRatioBinding{
			"new": {
				SourceType:    UpstreamGroupRatioBindingSourceChannel,
				SourceID:      3,
				UpstreamGroup: "new-upstream",
				Offset:        0.1,
			},
			"vip": {
				SourceType:    UpstreamGroupRatioBindingSourceChannel,
				SourceID:      1,
				UpstreamGroup: "vip-pro",
			},
		},
	)

	require.Equal(t, []UpstreamGroupRatioBindingChange{
		{
			Type:  UpstreamGroupRatioBindingChangeAdded,
			Group: "new",
			NewBinding: UpstreamGroupRatioBinding{
				SourceType:    UpstreamGroupRatioBindingSourceChannel,
				SourceID:      3,
				UpstreamGroup: "new-upstream",
				Offset:        0.1,
			},
		},
		{
			Type:  UpstreamGroupRatioBindingChangeDeleted,
			Group: "old",
			OldBinding: UpstreamGroupRatioBinding{
				SourceType:    UpstreamGroupRatioBindingSourceProvider,
				SourceID:      2,
				UpstreamGroup: "old-upstream",
			},
		},
		{
			Type:  UpstreamGroupRatioBindingChangeUpdated,
			Group: "vip",
			OldBinding: UpstreamGroupRatioBinding{
				SourceType:    UpstreamGroupRatioBindingSourceChannel,
				SourceID:      1,
				UpstreamGroup: "vip",
			},
			NewBinding: UpstreamGroupRatioBinding{
				SourceType:    UpstreamGroupRatioBindingSourceChannel,
				SourceID:      1,
				UpstreamGroup: "vip-pro",
			},
		},
	}, changes)
	require.Equal(t, []string{
		"+ new: channel #3 / new-upstream / 偏移 0.1",
		"- old: 原绑定 provider #2 / old-upstream",
		"* vip: channel #1 / vip -> channel #1 / vip-pro",
	}, FormatUpstreamGroupRatioBindingChangeLines(changes))
}

func TestCompareGroupSpecialUsableChanges(t *testing.T) {
	changes := CompareGroupSpecialUsableChanges(
		map[string]map[string]string{
			"vip": {
				"+:fast":  "Fast",
				"-:cheap": "Cheap",
			},
		},
		map[string]map[string]string{
			"vip": {
				"+:fast": "Fast Plus",
				"media":  "Media",
			},
		},
	)

	require.Equal(t, []GroupSpecialUsableChange{
		{
			Type:      GroupSpecialUsableChangeUpdated,
			UserGroup: "vip",
			Rule:      "+:fast",
			OldValue:  "Fast",
			NewValue:  "Fast Plus",
		},
		{
			Type:      GroupSpecialUsableChangeDeleted,
			UserGroup: "vip",
			Rule:      "-:cheap",
			OldValue:  "Cheap",
		},
		{
			Type:      GroupSpecialUsableChangeAdded,
			UserGroup: "vip",
			Rule:      "media",
			NewValue:  "Media",
		},
	}, changes)
	require.Equal(t, []string{
		"* 用户分组 vip: 可见 fast 描述 Fast -> Fast Plus",
		"- 用户分组 vip: 移除规则 不可见 cheap",
		"+ 用户分组 vip: 可见 media (Media)",
	}, FormatGroupSpecialUsableChangeLines(changes))
}
