package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestListModelsRecognizesTieredPricing(t *testing.T) {
	db := setupModelGroupCombinationControllerTest(t)
	operation_setting.SelfUseModeEnabled = false
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	originalRatios := ratio_setting.ModelRatio2JSONString()
	originalPrices := ratio_setting.ModelPrice2JSONString()
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	originalModes := billing_setting.GetBillingModeCopy()
	originalExpressions := billing_setting.GetBillingExprCopy()
	setBilling := func(modes, expressions map[string]string) {
		modeJSON, err := common.Marshal(modes)
		require.NoError(t, err)
		expressionJSON, err := common.Marshal(expressions)
		require.NoError(t, err)
		require.NoError(t, config.UpdateConfigFromMap(config.GlobalConfig.Get("billing_setting"), map[string]string{
			"billing_mode": string(modeJSON),
			"billing_expr": string(expressionJSON),
		}))
	}
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalPrices))
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
		setBilling(originalModes, originalExpressions)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-5.6":2.5}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"fixed-price-model":0}`))
	setBilling(map[string]string{
		"gpt-6.1-sol":             billing_setting.BillingModeTieredExpr,
		"gpt-6.1-sol-1m":          billing_setting.BillingModeTieredExpr,
		"tiered-no-expression":    billing_setting.BillingModeTieredExpr,
		"tiered-blank-expression": billing_setting.BillingModeTieredExpr,
	}, map[string]string{
		"gpt-6.1-sol":             `tier("standard", p * 2 + c * 10)`,
		"gpt-6.1-sol-1m":          `tier("standard", p * 2 + c * 10)`,
		"tiered-blank-expression": " \t\n ",
		"ratio-with-expression":   `tier("standard", p * 2)`,
	})

	models := []string{"gpt-6.1-sol", "gpt-6.1-sol-1m", "gpt-5.6", "fixed-price-model",
		"tiered-no-expression", "tiered-blank-expression", "ratio-with-expression", "unpriced-model"}
	for _, name := range models {
		if name != "gpt-5.6" {
			require.NoError(t, db.Create(&model.Ability{Group: "group-a", Model: name, ChannelId: 1, Enabled: true}).Error)
		}
	}
	members := []ratio_setting.GroupCombinationMember{
		{Group: "group-a", Models: models},
		{Group: "group-b", Models: []string{"shared-model"}},
	}
	memberJSON, err := common.Marshal(members)
	require.NoError(t, err)
	combinationJSON, err := common.Marshal(map[string][]ratio_setting.GroupCombinationMember{"tiered-combo": members})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(string(combinationJSON)))
	want := []string{"gpt-6.1-sol", "gpt-6.1-sol-1m", "gpt-5.6", "fixed-price-model"}

	for _, group := range []string{"group-a", "tiered-combo"} {
		t.Run(group, func(t *testing.T) {
			listed := buildUserOpenAIModels(model.GetGroupEnabledModels(group), false)
			ids := make([]string, 0, len(listed))
			for _, item := range listed {
				ids = append(ids, item.Id)
			}
			require.ElementsMatch(t, want, ids)
		})
	}
	for _, route := range []string{"token-combination", "token-model-limits", "token-combination-with-limits"} {
		t.Run(route, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			if route != "token-model-limits" {
				common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationEnabled, true)
				common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationGroups, string(memberJSON))
			}
			expected := want
			if route != "token-combination" {
				limits := make(map[string]bool, len(models))
				for _, name := range models {
					limits[name] = true
				}
				if route == "token-combination-with-limits" {
					limits = map[string]bool{"gpt-6.1-sol": true, "unselected-model": true}
					expected = []string{"gpt-6.1-sol"}
				}
				common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
				common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, limits)
			}
			ListModels(ctx, constant.ChannelTypeOpenAI)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.ElementsMatch(t, expected, decodeModelIDs(t, recorder))
		})
	}
}
