package setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisabledGroupDescriptionsAreStoredButNotSelectable(t *testing.T) {
	original := UserUsableGroups2JSONString()
	defer func() {
		if err := UpdateUserUsableGroupsByJSONString(original); err != nil {
			t.Fatalf("restore user usable groups: %v", err)
		}
	}()

	err := UpdateUserUsableGroupsByJSONString(`{"default":"Default","__disabled_description__:vip":"VIP note"}`)
	if err != nil {
		t.Fatalf("update user usable groups: %v", err)
	}

	groups := GetUserUsableGroupsCopy()
	if groups["default"] != "Default" {
		t.Fatalf("expected default group description, got %#v", groups["default"])
	}
	if _, ok := groups["__disabled_description__:vip"]; ok {
		t.Fatal("disabled group description key should not be exposed as selectable")
	}

	raw := UserUsableGroups2JSONString()
	if !strings.Contains(raw, "__disabled_description__:vip") {
		t.Fatalf("expected disabled group description to remain stored, got %s", raw)
	}
}

func TestBuildDisabledUserUsableGroupJSONString(t *testing.T) {
	original := UserUsableGroups2JSONString()
	defer func() {
		if err := UpdateUserUsableGroupsByJSONString(original); err != nil {
			t.Fatalf("restore user usable groups: %v", err)
		}
	}()

	require.NoError(t, UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))

	next, changed, err := BuildDisabledUserUsableGroupJSONString("vip")
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, `{"default":"Default","__disabled_description__:vip":"VIP"}`, next)

	next, changed, err = BuildDisabledUserUsableGroupJSONString("missing")
	require.NoError(t, err)
	require.False(t, changed)
	require.Empty(t, next)
}

func TestCompareUserUsableGroupChanges(t *testing.T) {
	changes := CompareUserUsableGroupChanges(
		map[string]string{
			"default":                           "Default",
			"vip":                               "VIP",
			"__disabled_description__:disabled": "Disabled",
			"removed":                           "Removed",
		},
		map[string]string{
			"default":                         "Default Plus",
			"__disabled_description__:vip":    "VIP",
			"disabled":                        "Disabled",
			"__disabled_description__:hidden": "Hidden",
		},
	)

	require.Equal(t, []UserUsableGroupChange{
		{
			Type:    UserUsableGroupChangeDescription,
			Group:   "default",
			OldDesc: "Default",
			NewDesc: "Default Plus",
		},
		{
			Type:        UserUsableGroupChangeUpdated,
			Group:       "disabled",
			OldDesc:     "Disabled",
			NewDesc:     "Disabled",
			OldDisabled: true,
		},
		{
			Type:        UserUsableGroupChangeAdded,
			Group:       "hidden",
			NewDesc:     "Hidden",
			NewDisabled: true,
		},
		{
			Type:    UserUsableGroupChangeDeleted,
			Group:   "removed",
			OldDesc: "Removed",
		},
		{
			Type:        UserUsableGroupChangeUpdated,
			Group:       "vip",
			OldDesc:     "VIP",
			NewDesc:     "VIP",
			NewDisabled: true,
		},
	}, changes)
	require.Equal(t, []string{
		"* default: 描述 Default -> Default Plus",
		"* disabled: 关闭 -> 开启，描述 Disabled，倍率 2",
		"- removed: 关闭",
		"* vip: 开启 -> 关闭",
	}, FormatUserUsableGroupChangeLines(changes, map[string]float64{"disabled": 2}))
}
