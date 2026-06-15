package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
