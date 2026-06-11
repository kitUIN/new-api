package operation_setting

import "testing"

func TestParseAutoTestChannelSkipGroups(t *testing.T) {
	groups := ParseAutoTestChannelSkipGroups("default, vip\n test，internal； ;")

	for _, group := range []string{"default", "vip", "test", "internal"} {
		if _, ok := groups[group]; !ok {
			t.Fatalf("expected group %q to be parsed", group)
		}
	}
	if _, ok := groups[""]; ok {
		t.Fatal("empty group should not be parsed")
	}
}
