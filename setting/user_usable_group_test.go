package setting

import (
	"strings"
	"testing"
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
