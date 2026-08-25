package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildChannelAffinityTemplateContextForTest(meta channelAffinityMeta) *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	setChannelAffinityContext(ctx, meta)
	return ctx
}

func TestAppendChannelAffinitySessionKeyAdminInfo(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName:       "codex-session",
		UsingGroup:     "default",
		ModelName:      "gpt-5",
		RequestPath:    "/v1/responses",
		KeySourceType:  "gjson",
		KeySourcePath:  "prompt_cache_key",
		KeyHint:        "abcd...wxyz",
		KeyFingerprint: "fp123456",
	})
	adminInfo := map[string]interface{}{}

	AppendChannelAffinitySessionKeyAdminInfo(ctx, adminInfo)

	sessionKey, ok := adminInfo["session_key"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "codex-session", sessionKey["rule_name"])
	require.Equal(t, "abcd...wxyz", sessionKey["key_hint"])
	require.Equal(t, "fp123456", sessionKey["key_fp"])
	require.NotContains(t, adminInfo, "channel_affinity")
}

func TestEnsureChannelAffinitySessionKeyWhenAffinityDisabled(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	enabled := setting.Enabled
	setting.Enabled = false
	defer func() {
		setting.Enabled = enabled
	}()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"prompt_cache_key":"session-abc"}`))

	EnsureChannelAffinitySessionKey(ctx, "gpt-5", "default")
	adminInfo := map[string]interface{}{}
	AppendChannelAffinitySessionKeyAdminInfo(ctx, adminInfo)

	sessionKey, ok := adminInfo["session_key"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "codex cli trace", sessionKey["rule_name"])
	require.Equal(t, "session-abc", sessionKey["key_hint"])

	merged, applied := ApplyChannelAffinityOverrideTemplate(ctx, map[string]interface{}{"temperature": 0.7})
	require.False(t, applied)
	require.Equal(t, 0.7, merged["temperature"])

	_, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")
	require.False(t, found)
}

func TestApplyChannelAffinityOverrideTemplate_NoTemplate(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName: "rule-no-template",
	})
	base := map[string]interface{}{
		"temperature": 0.7,
	}

	merged, applied := ApplyChannelAffinityOverrideTemplate(ctx, base)
	require.False(t, applied)
	require.Equal(t, base, merged)
}

func TestApplyChannelAffinityOverrideTemplate_MergeTemplate(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName: "rule-with-template",
		ParamTemplate: map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.95,
		},
		UsingGroup:     "default",
		ModelName:      "gpt-4.1",
		RequestPath:    "/v1/responses",
		KeySourceType:  "gjson",
		KeySourcePath:  "prompt_cache_key",
		KeyHint:        "abcd...wxyz",
		KeyFingerprint: "abcd1234",
	})
	base := map[string]interface{}{
		"temperature": 0.7,
		"max_tokens":  2000,
	}

	merged, applied := ApplyChannelAffinityOverrideTemplate(ctx, base)
	require.True(t, applied)
	require.Equal(t, 0.7, merged["temperature"])
	require.Equal(t, 0.95, merged["top_p"])
	require.Equal(t, 2000, merged["max_tokens"])
	require.Equal(t, 0.7, base["temperature"])

	anyInfo, ok := ctx.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info, ok := anyInfo.(map[string]interface{})
	require.True(t, ok)
	overrideInfoAny, ok := info["override_template"]
	require.True(t, ok)
	overrideInfo, ok := overrideInfoAny.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, true, overrideInfo["applied"])
	require.Equal(t, "rule-with-template", overrideInfo["rule_name"])
	require.EqualValues(t, 2, overrideInfo["param_override_keys"])
}

func TestApplyChannelAffinityOverrideTemplate_MergeOperations(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName: "rule-with-ops-template",
		ParamTemplate: map[string]interface{}{
			"operations": []map[string]interface{}{
				{
					"mode":  "pass_headers",
					"value": []string{"Originator"},
				},
			},
		},
	})
	base := map[string]interface{}{
		"temperature": 0.7,
		"operations": []map[string]interface{}{
			{
				"path":  "model",
				"mode":  "trim_prefix",
				"value": "openai/",
			},
		},
	}

	merged, applied := ApplyChannelAffinityOverrideTemplate(ctx, base)
	require.True(t, applied)
	require.Equal(t, 0.7, merged["temperature"])

	opsAny, ok := merged["operations"]
	require.True(t, ok)
	ops, ok := opsAny.([]interface{})
	require.True(t, ok)
	require.Len(t, ops, 2)

	firstOp, ok := ops[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "pass_headers", firstOp["mode"])

	secondOp, ok := ops[1].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "trim_prefix", secondOp["mode"])
}

func TestShouldSkipRetryAfterChannelAffinityFailure(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() *gin.Context
		want bool
	}{
		{
			name: "nil context",
			ctx: func() *gin.Context {
				return nil
			},
			want: false,
		},
		{
			name: "explicit skip retry flag in context",
			ctx: func() *gin.Context {
				ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
					RuleName:   "rule-explicit-flag",
					SkipRetry:  false,
					UsingGroup: "default",
					ModelName:  "gpt-5",
				})
				ctx.Set(ginKeyChannelAffinitySkipRetry, true)
				return ctx
			},
			want: true,
		},
		{
			name: "fallback to matched rule meta",
			ctx: func() *gin.Context {
				return buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
					RuleName:   "rule-skip-retry",
					SkipRetry:  true,
					UsingGroup: "default",
					ModelName:  "gpt-5",
				})
			},
			want: true,
		},
		{
			name: "no flag and no skip retry meta",
			ctx: func() *gin.Context {
				return buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
					RuleName:   "rule-no-skip-retry",
					SkipRetry:  false,
					UsingGroup: "default",
					ModelName:  "gpt-5",
				})
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ShouldSkipRetryAfterChannelAffinityFailure(tt.ctx()))
		})
	}
}

func TestShouldSkipRetryAfterChannelAffinityFailureAllowsAutoGroupFailover(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName:   "rule-skip-retry",
		SkipRetry:  true,
		UsingGroup: "auto",
		ModelName:  "gpt-5",
	})
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "auto")

	require.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))
}

func TestShouldSkipRetryAfterChannelAffinityFailureAllowsGroupCombinationFailover(t *testing.T) {
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"enhanced":[{"group":"preferred","models":["gpt-5"]},{"group":"enhanced","models":["gpt-5"]}]}`,
	))

	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName:   "rule-skip-retry",
		SkipRetry:  true,
		UsingGroup: "enhanced",
		ModelName:  "gpt-5",
	})
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "preferred")

	require.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))
}

func TestPrepareAutoGroupAffinityFailoverAdvancesToNextGroup(t *testing.T) {
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"g1":"G1","g2":"G2"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"g1":1,"g2":2}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["g1","g2"]`))

	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		RuleName:   "rule-auto",
		UsingGroup: "auto",
		ModelName:  "gpt-5",
	})
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "auto")
	common.SetContextKey(ctx, constant.ContextKeyAutoGroup, "g1")

	retry := 1
	retryParam := &RetryParam{
		Ctx:        ctx,
		TokenGroup: "auto",
		ModelName:  "gpt-5",
		Retry:      &retry,
	}

	require.True(t, PrepareAutoGroupAffinityFailover(ctx, retryParam))
	require.Equal(t, 0, retryParam.GetRetry())
	require.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyAutoGroupIndex))
}

func TestAutoGroupAffinityUsesResetSecondsAsDefaultTTL(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	originalDefaultTTL := setting.DefaultTTLSeconds
	originalAutoReset := setting.AutoGroupResetSeconds
	t.Cleanup(func() {
		setting.DefaultTTLSeconds = originalDefaultTTL
		setting.AutoGroupResetSeconds = originalAutoReset
	})
	setting.DefaultTTLSeconds = 3600
	setting.AutoGroupResetSeconds = 18000

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"prompt_cache_key":"session-abc"}`))

	EnsureChannelAffinitySessionKey(ctx, "gpt-5", "auto")
	_, ttlSeconds, ok := getChannelAffinityContext(ctx)

	require.True(t, ok)
	require.Equal(t, 18000, ttlSeconds)
}

func TestChannelAffinityHitCodexTemplatePassHeadersEffective(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, setting)

	var codexRule *operation_setting.ChannelAffinityRule
	for i := range setting.Rules {
		rule := &setting.Rules[i]
		if strings.EqualFold(strings.TrimSpace(rule.Name), "codex cli trace") {
			codexRule = rule
			break
		}
	}
	require.NotNil(t, codexRule)

	affinityValue := fmt.Sprintf("pc-hit-%d", time.Now().UnixNano())
	cacheKeySuffix := buildChannelAffinityCacheKeySuffix(*codexRule, "gpt-5", "default", affinityValue)

	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, 9527, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(fmt.Sprintf(`{"prompt_cache_key":"%s"}`, affinityValue)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-5", "default")
	require.True(t, found)
	require.Equal(t, 9527, channelID)

	baseOverride := map[string]interface{}{
		"temperature": 0.2,
	}
	mergedOverride, applied := ApplyChannelAffinityOverrideTemplate(ctx, baseOverride)
	require.True(t, applied)
	require.Equal(t, 0.2, mergedOverride["temperature"])

	info := &relaycommon.RelayInfo{
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
			"User-Agent": "codex-cli-test",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: mergedOverride,
			HeadersOverride: map[string]interface{}{
				"X-Static": "legacy-static",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-5"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)

	require.Equal(t, "legacy-static", info.RuntimeHeadersOverride["x-static"])
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	require.Equal(t, "codex-cli-test", info.RuntimeHeadersOverride["user-agent"])

	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	_, exists = info.RuntimeHeadersOverride["x-codex-turn-metadata"]
	require.False(t, exists)
}
