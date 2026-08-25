package helper

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func updateBillingSettingForTest(t *testing.T, modes map[string]string, exprs map[string]string) {
	t.Helper()
	cfg := config.GlobalConfig.Get("billing_setting")
	require.NotNil(t, cfg)

	modeBytes, err := common.Marshal(modes)
	require.NoError(t, err)
	exprBytes, err := common.Marshal(exprs)
	require.NoError(t, err)
	require.NoError(t, config.UpdateConfigFromMap(cfg, map[string]string{
		"billing_mode": string(modeBytes),
		"billing_expr": string(exprBytes),
	}))
}

func TestModelPriceHelperTieredUsesFinalGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldModes := billing_setting.GetBillingModeCopy()
	oldExprs := billing_setting.GetBillingExprCopy()
	t.Cleanup(func() {
		updateBillingSettingForTest(t, oldModes, oldExprs)
	})

	modelName := "tiered-group-test-model"
	expr := `group == "vip" ? tier("vip", p * 1) : tier("base", p * 2)`
	updateBillingSettingForTest(t, map[string]string{
		modelName: billing_setting.BillingModeTieredExpr,
	}, map[string]string{
		modelName: expr,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(bytes.NewReader(body)))
	ctx.Set(common.KeyRequestBody, body)
	ctx.Set("auto_group", "vip")

	info := &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, "vip", info.UsingGroup)
	require.NotNil(t, info.BillingRequestInput)
	require.Equal(t, "vip", info.BillingRequestInput.Group)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "vip", info.TieredBillingSnapshot.Group)
	require.Equal(t, "vip", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, priceData.QuotaToPreConsume, info.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
}

func TestRefreshPricingForSelectedGroupReevaluatesTieredExpr(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldModes := billing_setting.GetBillingModeCopy()
	oldExprs := billing_setting.GetBillingExprCopy()
	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		updateBillingSettingForTest(t, oldModes, oldExprs)
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldGroupRatios))
	})

	modelName := "tiered-group-switch-test-model"
	expr := `group == "premium" ? tier("premium", p * 2) : tier("combo", p)`
	updateBillingSettingForTest(t, map[string]string{
		modelName: billing_setting.BillingModeTieredExpr,
	}, map[string]string{
		modelName: expr,
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"combo":3,"premium":0.5}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{}`))

	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(bytes.NewReader(body)))
	ctx.Set(common.KeyRequestBody, body)

	info := &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UserGroup:       "default",
		UsingGroup:      "combo",
		RequestHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	}
	const promptTokens = 1000
	priceData, err := ModelPriceHelper(ctx, info, promptTokens, &types.TokenCountMeta{})
	require.NoError(t, err)
	initialPreConsumedQuota := priceData.QuotaToPreConsume
	require.Equal(t, "combo", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, 3.0, info.TieredBillingSnapshot.GroupRatio)

	require.NoError(t, RefreshPricingForSelectedGroup(ctx, info, "premium"))
	expectedBeforeGroup := float64(promptTokens*2) / 1_000_000 * info.TieredBillingSnapshot.QuotaPerUnit
	require.Equal(t, "premium", info.UsingGroup)
	require.Equal(t, "premium", info.BillingRequestInput.Group)
	require.Equal(t, "premium", info.TieredBillingSnapshot.Group)
	require.Equal(t, "premium", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, 0.5, info.TieredBillingSnapshot.GroupRatio)
	require.Equal(t, 0.5, info.PriceData.GroupRatioInfo.GroupRatio)
	require.InDelta(t, expectedBeforeGroup, info.TieredBillingSnapshot.EstimatedQuotaBeforeGroup, 0.000001)
	require.Equal(t, billingexpr.QuotaRound(expectedBeforeGroup*0.5), info.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
	require.Equal(t, initialPreConsumedQuota, info.PriceData.QuotaToPreConsume)
	require.NotEqual(t, initialPreConsumedQuota, info.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
}

func newOpenAIFastPricingContext(body []byte) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(bytes.NewReader(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(common.KeyRequestBody, body)
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
	common.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
		AllowServiceTier: true,
	})
	return ctx
}

func TestModelPriceHelperOpenAIFastModeAppliesTwoPointFiveMultiplier(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldModes := billing_setting.GetBillingModeCopy()
	oldExprs := billing_setting.GetBillingExprCopy()
	t.Cleanup(func() {
		updateBillingSettingForTest(t, oldModes, oldExprs)
	})
	updateBillingSettingForTest(t, map[string]string{}, map[string]string{})

	newRelayInfo := func(tier string) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			OriginModelName: "gpt-4.1",
			UserGroup:       "default",
			UsingGroup:      "default",
			ServiceTier:     tier,
			UserSetting: dto.UserSetting{
				AcceptUnsetRatioModel: true,
			},
		}
	}

	standardInfo := newRelayInfo("default")
	standardPrice, err := ModelPriceHelper(
		newOpenAIFastPricingContext([]byte(`{"service_tier":"default"}`)),
		standardInfo,
		1000,
		&types.TokenCountMeta{},
	)
	require.NoError(t, err)
	require.Greater(t, standardPrice.QuotaToPreConsume, 0)

	fastInfo := newRelayInfo("fast")
	fastPrice, err := ModelPriceHelper(
		newOpenAIFastPricingContext([]byte(`{"service_tier":"fast"}`)),
		fastInfo,
		1000,
		&types.TokenCountMeta{},
	)
	require.NoError(t, err)
	require.Equal(t, 2.5, fastInfo.GetServiceTierBillingMultiplier())
	require.Equal(t, int(float64(standardPrice.QuotaToPreConsume)*2.5), fastPrice.QuotaToPreConsume)
}

func TestModelPriceHelperTieredFastModeDoesNotStackExistingTierRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldModes := billing_setting.GetBillingModeCopy()
	oldExprs := billing_setting.GetBillingExprCopy()
	t.Cleanup(func() {
		updateBillingSettingForTest(t, oldModes, oldExprs)
	})

	modelName := "tiered-fast-mode-test-model"
	expr := `tier("fast", p * 1) * (param("service_tier") == "fast" ? 2 : 1)`
	updateBillingSettingForTest(t, map[string]string{
		modelName: billing_setting.BillingModeTieredExpr,
	}, map[string]string{
		modelName: expr,
	})

	body := []byte(`{"service_tier":"fast","messages":[{"role":"user","content":"hi"}]}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UserGroup:       "default",
		UsingGroup:      "default",
		ServiceTier:     "fast",
		RequestHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	}

	const promptTokens = 1000
	priceData, err := ModelPriceHelper(
		newOpenAIFastPricingContext(body),
		info,
		promptTokens,
		&types.TokenCountMeta{},
	)
	require.NoError(t, err)
	require.NotNil(t, info.BillingRequestInput)
	require.False(t, gjson.GetBytes(info.BillingRequestInput.Body, "service_tier").Exists())
	require.NotNil(t, info.TieredBillingSnapshot)

	expectedBeforeGroup := float64(promptTokens) / 1_000_000 * common.QuotaPerUnit * relaycommon.OpenAIFastModeBillingMultiplier
	require.InDelta(t, expectedBeforeGroup, info.TieredBillingSnapshot.EstimatedQuotaBeforeGroup, 0.000001)
	require.Equal(
		t,
		billingexpr.QuotaRound(expectedBeforeGroup*info.TieredBillingSnapshot.GroupRatio),
		priceData.QuotaToPreConsume,
	)
}
