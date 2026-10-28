package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestFilterPricingByUsableGroups(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		usable map[string]string
		want   []string
	}{
		{
			name:   "remove unavailable groups from a visible model",
			groups: []string{"disabled", "enabled", "hidden", "authorized"},
			usable: map[string]string{"enabled": "", "authorized": ""},
			want:   []string{"enabled", "authorized"},
		},
		{
			name:   "omit models with only unavailable groups",
			groups: []string{"disabled"},
			usable: map[string]string{"enabled": ""},
		},
		{
			name:   "omit models without groups",
			usable: map[string]string{"enabled": ""},
		},
		{
			name:   "preserve available group order",
			groups: []string{"second", "first"},
			usable: map[string]string{"first": "", "second": ""},
			want:   []string{"second", "first"},
		},
		{
			name:   "preserve wildcard while removing unavailable named groups",
			groups: []string{"disabled", "all", "enabled"},
			usable: map[string]string{"enabled": ""},
			want:   []string{"all", "enabled"},
		},
		{
			name:   "wildcard does not require a named group match",
			groups: []string{"all"},
			usable: map[string]string{"enabled": ""},
			want:   []string{"all"},
		},
		{
			name:   "no usable groups hides wildcard models too",
			groups: []string{"all", "enabled"},
			usable: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := model.Pricing{
				ModelName:   "shared-model",
				Description: "model description",
				ModelRatio:  2,
				EnableGroup: tt.groups,
			}
			pricing := []model.Pricing{original}
			got := filterPricingByUsableGroups(pricing, tt.usable)
			if len(tt.want) == 0 {
				require.Empty(t, got)
			} else {
				expected := original
				expected.EnableGroup = tt.want
				require.Equal(t, []model.Pricing{expected}, got)
			}
			require.Equal(t, []model.Pricing{original}, pricing)
		})
	}

	require.Nil(t, filterPricingByUsableGroups(nil, map[string]string{"enabled": ""}))
	require.Equal(t, []model.Pricing{}, filterPricingByUsableGroups([]model.Pricing{}, nil))
}

func TestFilterPricingByUsableGroupsPreservesSharedCache(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "shared", EnableGroup: []string{"hidden", "group-a", "group-b"}},
		{ModelName: "only-b", EnableGroup: []string{"group-b"}},
	}

	forUserA := filterPricingByUsableGroups(pricing, map[string]string{"group-a": ""})
	require.Equal(t, []model.Pricing{
		{ModelName: "shared", EnableGroup: []string{"group-a"}},
	}, forUserA)

	// The response must not share a writable group slice with the global cache.
	forUserA[0].EnableGroup[0] = "changed-in-response"
	require.Equal(t, []string{"hidden", "group-a", "group-b"}, pricing[0].EnableGroup)

	forUserB := filterPricingByUsableGroups(pricing, map[string]string{"group-b": ""})
	require.Equal(t, []model.Pricing{
		{ModelName: "shared", EnableGroup: []string{"group-b"}},
		{ModelName: "only-b", EnableGroup: []string{"group-b"}},
	}, forUserB)
}
