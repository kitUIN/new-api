package helper

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
