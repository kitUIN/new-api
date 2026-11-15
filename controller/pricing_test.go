package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetPricingExcludesUserGroups(t *testing.T) {
	db := setupModelGroupCombinationControllerTest(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		model.InvalidatePricingCache()
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Model{}, &model.Vendor{}, &model.User{}))

	for _, config := range []struct {
		original string
		value    string
		update   func(string) error
	}{
		{setting.UserUsableGroups2JSONString(), `{"group-a":"Billing","group-b":"User","free":"Free"}`, setting.UpdateUserUsableGroupsByJSONString},
		{ratio_setting.GroupRatio2JSONString(), `{"group-a":2,"group-b":1,"user-own":1,"free":1}`, ratio_setting.UpdateGroupRatioByJSONString},
		{ratio_setting.GroupTypes2JSONString(), `{"group-b":"user","user-own":"user","free":"billing"}`, ratio_setting.UpdateGroupTypesByJSONString},
		{ratio_setting.GroupGroupRatio2JSONString(), `{"user-own":{"group-a":1.25}}`, ratio_setting.UpdateGroupGroupRatioByJSONString},
		{setting.AutoGroups2JsonString(), `["group-b","group-a","user-own"]`, setting.UpdateAutoGroupsByJsonString},
		{ratio_setting.GroupCombinations2JSONString(), `{}`, ratio_setting.UpdateGroupCombinationsByJSONString},
	} {
		t.Cleanup(func() { require.NoError(t, config.update(config.original)) })
		require.NoError(t, config.update(config.value))
	}
	special := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	oldSpecial := special.ReadAll()
	special.Clear()
	t.Cleanup(func() { special.Clear(); special.AddAll(oldSpecial) })
	timed := ratio_setting.GetGroupRatioSetting().GroupTimedRatios
	oldTimed, err := common.Marshal(timed)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, common.Unmarshal(oldTimed, timed)) })
	require.NoError(t, common.UnmarshalJsonStr(`{
		"group-b":[{"enabled":true,"ratio":0.2,"kind":"daily","start":"00:00","end":"24:00"}],
		"user-own":[{"enabled":true,"ratio":0.3,"kind":"daily","start":"00:00","end":"24:00"}],
		"free":[{"enabled":true,"ratio":0,"kind":"daily","start":"00:00","end":"24:00"}]
	}`, timed))
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedis })
	user := model.User{Username: "pricing-user", Group: "user-own"}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "user-own", Model: "own-only", ChannelId: 1, Enabled: true}).Error)
	model.RefreshPricing()

	for _, tc := range []struct {
		name  string
		id    int
		ratio float64
	}{
		{"anonymous", 0, 2},
		{"authenticated", user.Id, 1.25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
			if tc.id != 0 {
				ctx.Set("id", tc.id)
			}
			GetPricing(ctx)
			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success         bool                                      `json:"success"`
				Data            []model.Pricing                           `json:"data"`
				UsableGroup     map[string]string                         `json:"usable_group"`
				GroupRatio      map[string]float64                        `json:"group_ratio"`
				GroupTimedRatio map[string]*types.GroupTimedRatioSnapshot `json:"group_timed_ratio"`
				AutoGroups      []string                                  `json:"auto_groups"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success)
			require.Equal(t, map[string]string{"group-a": "Billing", "free": "Free"}, response.UsableGroup)
			require.Equal(t, map[string]float64{"group-a": tc.ratio, "free": 0}, response.GroupRatio)
			require.Len(t, response.GroupTimedRatio, 1)
			require.NotNil(t, response.GroupTimedRatio["free"])
			require.Equal(t, 1.0, response.GroupTimedRatio["free"].OriginalRatio)
			require.Equal(t, []string{"group-a"}, response.AutoGroups)
			require.Len(t, response.Data, 2)
			for _, item := range response.Data {
				require.Equal(t, []string{"group-a"}, item.EnableGroup)
			}
		})
	}

	// Filtering the public response must not change routing data in the shared cache.
	for _, item := range model.GetPricing() {
		if item.ModelName == "shared-model" {
			require.ElementsMatch(t, []string{"group-a", "group-b"}, item.EnableGroup)
		}
	}
}

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
