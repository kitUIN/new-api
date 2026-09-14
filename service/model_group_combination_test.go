package service

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelGroupCombinationSettings(t *testing.T) {
	t.Helper()
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatio))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"group-a":"A","group-b":"B","group-c":"C","deprecated":"Deprecated"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"group-a":1,"group-b":2,"group-c":3}`))
}

func TestNormalizeTokenModelGroupCombination(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	token := &model.Token{
		Group:                        "auto",
		CrossGroupRetry:              true,
		AutoGroupMode:                RuleAutoGroupModeBalanced,
		ModelGroupCombinationEnabled: true,
		ModelGroupCombinationGroups:  `[{"group":" group-a ","models":[" shared-model "]},{"group":"group-b","models":["deepseek-v4-flash"]}]`,
	}

	require.NoError(t, NormalizeTokenModelGroupCombination(token, ""))
	require.Equal(t, "group-a", token.Group)
	require.False(t, token.CrossGroupRetry)
	require.Empty(t, token.AutoGroupMode)
	require.JSONEq(t, `[{"group":"group-a","models":["shared-model"]},{"group":"group-b","models":["deepseek-v4-flash"]}]`, token.ModelGroupCombinationGroups)
}

func TestNormalizeTokenModelGroupCombinationRejectsInvalidConfiguration(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	tests := []struct {
		name     string
		groups   string
		failover bool
	}{
		{name: "single group", groups: `[{"group":"group-a","models":["shared-model"]}]`},
		{name: "duplicate group", groups: `[{"group":"group-a","models":["shared-model"]},{"group":"group-a","models":["shared-model"]}]`},
		{name: "auto group", groups: `[{"group":"group-a","models":["shared-model"]},{"group":"auto","models":["shared-model"]}]`},
		{name: "rule auto group", groups: `[{"group":"group-a","models":["shared-model"]},{"group":"自动分组:codex","models":["shared-model"]}]`},
		{name: "unauthorized group", groups: `[{"group":"group-a","models":["shared-model"]},{"group":"missing","models":["shared-model"]}]`},
		{name: "deprecated group", groups: `[{"group":"group-a","models":["shared-model"]},{"group":"deprecated","models":["shared-model"]}]`},
		{name: "missing models", groups: `[{"group":"group-a","models":[]},{"group":"group-b","models":["shared-model"]}]`},
		{name: "duplicate models", groups: `[{"group":"group-a","models":["shared-model","shared-model"]},{"group":"group-b","models":["shared-model"]}]`},
		{name: "invalid json", groups: `{`},
		{name: "failover enabled", groups: `[{"group":"group-a","models":["shared-model"]},{"group":"group-b","models":["shared-model"]}]`, failover: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &model.Token{
				ModelGroupCombinationEnabled: true,
				ModelGroupCombinationGroups:  tt.groups,
				SessionGroupFailoverEnabled:  tt.failover,
			}
			require.Error(t, NormalizeTokenModelGroupCombination(token, ""))
		})
	}
}

func TestNormalizeTokenModelGroupCombinationRejectsNestedCombination(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"group-c":[{"group":"group-a","models":["shared-model"]},{"group":"group-b","models":["shared-model"]}]}`,
	))

	token := &model.Token{
		ModelGroupCombinationEnabled: true,
		ModelGroupCombinationGroups:  `[{"group":"group-a","models":["shared-model"]},{"group":"group-c","models":["shared-model"]}]`,
	}
	require.ErrorContains(t, NormalizeTokenModelGroupCombination(token, ""), "模型组合不支持嵌套组合分组")
}

func TestResolveModelGroupCombinationUsesConfiguredOrderAndAvailability(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	require.NoError(t, db.Create([]model.Ability{
		{Group: "group-a", Model: "shared-model", ChannelId: 1, Enabled: true},
		{Group: "group-b", Model: "shared-model", ChannelId: 2, Enabled: true},
		{Group: "group-b", Model: "deepseek-v4-flash", ChannelId: 2, Enabled: true},
		{Group: "group-c", Model: "shared-model", ChannelId: 3, Enabled: true},
	}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationGroups, `[{"group":"group-a","models":["gpt-5.6"]},{"group":"group-b","models":["shared-model","deepseek-v4-flash"]},{"group":"group-c","models":["shared-model"]}]`)

	group, enabled, err := ResolveModelGroupCombination(ctx, "shared-model")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "group-b", group)
	require.Equal(t, "group-b", common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))

	group, enabled, err = ResolveModelGroupCombination(ctx, "deepseek-v4-flash")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "group-b", group)

	_, enabled, err = ResolveModelGroupCombination(ctx, "missing-model")
	require.True(t, enabled)
	require.Error(t, err)
}

func TestResolveModelGroupCombinationAcceptsLegacyGroupArray(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	setupModelGroupCombinationDatabase(t)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationGroups, `["group-a","group-b"]`)

	group, enabled, err := ResolveModelGroupCombination(ctx, "shared-model")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "group-b", group)
}

func setupModelGroupCombinationDatabase(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	require.NoError(t, db.Create(&model.Ability{
		Group: "group-b", Model: "shared-model", ChannelId: 2, Enabled: true,
	}).Error)
}

func TestModelGroupCombinationUsesSharedFailoverRuntime(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	priority := int64(10)
	channels := []model.Channel{
		{Id: 1, Name: "group-a", Key: "sk-a", Status: common.ChannelStatusEnabled, Models: "shared-model", Group: "group-a", Priority: &priority},
		{Id: 2, Name: "group-b", Key: "sk-b", Status: common.ChannelStatusEnabled, Models: "shared-model", Group: "group-b", Priority: &priority},
	}
	for i := range channels {
		require.NoError(t, db.Create(&channels[i]).Error)
		require.NoError(t, channels[i].AddAbilities(nil))
	}

	rawMembers := `[{"group":"group-a","models":["shared-model"]},{"group":"group-b","models":["shared-model"]}]`
	members, _, err := ParseModelGroupCombinationMembers(rawMembers)
	require.NoError(t, err)
	_, breakerScope, _, err := modelGroupCombinationRuntimeIdentity(42, members)
	require.NoError(t, err)
	groupCombinationBreakerMemoryMu.Lock()
	delete(groupCombinationBreakerMemory, groupCombinationBreakerScopedStateKey(breakerScope, "group-a"))
	delete(groupCombinationBreakerMemory, groupCombinationBreakerScopedStateKey(breakerScope, "group-b"))
	groupCombinationBreakerMemoryMu.Unlock()
	t.Cleanup(func() {
		groupCombinationBreakerMemoryMu.Lock()
		delete(groupCombinationBreakerMemory, groupCombinationBreakerScopedStateKey(breakerScope, "group-a"))
		delete(groupCombinationBreakerMemory, groupCombinationBreakerScopedStateKey(breakerScope, "group-b"))
		groupCombinationBreakerMemoryMu.Unlock()
	})

	newContext := func(tokenID int, sessionSuffix string) *gin.Context {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		common.SetContextKey(ctx, constant.ContextKeyTokenId, tokenID)
		common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "group-a")
		common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "group-a")
		common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationEnabled, true)
		common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationGroups, rawMembers)
		if sessionSuffix != "" {
			setChannelAffinityContext(ctx, channelAffinityMeta{
				CacheKeySuffix: sessionSuffix,
				TTLSeconds:     60,
			})
		}
		return ctx
	}

	sessionSuffix := fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
	ctx := newContext(42, sessionSuffix)
	enabled, err := PrepareModelGroupCombination(ctx, "shared-model")
	require.NoError(t, err)
	require.True(t, enabled)
	retryParam := &RetryParam{
		Ctx: ctx, TokenGroup: "group-a", ModelName: "shared-model", Retry: common.GetPointer(0),
	}
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(retryParam)
	require.NoError(t, err)
	require.Equal(t, 1, channel.Id)
	require.Equal(t, "group-a", selectedGroup)
	require.True(t, PrepareGroupCombinationFailover(ctx, retryParam))
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(retryParam)
	require.NoError(t, err)
	require.Equal(t, 2, channel.Id)
	require.Equal(t, "group-b", selectedGroup)
	require.Equal(t, "group-b", common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))

	runtimeValue, ok := ctx.Get(ginKeyGroupCombinationRuntime)
	require.True(t, ok)
	cacheKey := runtimeValue.(*groupCombinationRuntime).CacheKey
	t.Cleanup(func() {
		if cacheKey != "" {
			_, _ = getGroupCombinationSessionCache().DeleteMany([]string{cacheKey})
		}
	})

	nextCtx := newContext(42, sessionSuffix)
	resolvedGroup, enabled, err := ResolveModelGroupCombination(nextCtx, "shared-model")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "group-b", resolvedGroup)
	require.Equal(t, "group-b", common.GetContextKeyString(nextCtx, constant.ContextKeyTokenGroup))
	_, runtimePrepared := nextCtx.Get(ginKeyGroupCombinationRuntime)
	require.False(t, runtimePrepared)
	_, err = PrepareModelGroupCombination(nextCtx, "shared-model")
	require.NoError(t, err)
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: nextCtx, TokenGroup: "group-a", ModelName: "shared-model", Retry: common.GetPointer(0),
	})
	require.NoError(t, err)
	require.Equal(t, 2, channel.Id)
	require.Equal(t, "group-b", selectedGroup)

	for failure := 1; failure < GroupCombinationBreakerFailureThreshold; failure++ {
		failureCtx := newContext(42, "")
		_, err = PrepareModelGroupCombination(failureCtx, "shared-model")
		require.NoError(t, err)
		failureRetryParam := &RetryParam{
			Ctx: failureCtx, TokenGroup: "group-a", ModelName: "shared-model", Retry: common.GetPointer(0),
		}
		channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(failureRetryParam)
		require.NoError(t, err)
		require.Equal(t, "group-a", selectedGroup)
		require.True(t, PrepareGroupCombinationFailover(failureCtx, failureRetryParam))
	}

	token := &model.Token{Id: 42, ModelGroupCombinationEnabled: true, ModelGroupCombinationGroups: rawMembers}
	summary, err := GetModelGroupCombinationCircuitBreakerSummary(token)
	require.NoError(t, err)
	require.Equal(t, GroupCombinationBreakerStatusSkipped, summary.Groups[0].Status)

	skippedCtx := newContext(42, "")
	_, err = PrepareModelGroupCombination(skippedCtx, "shared-model")
	require.NoError(t, err)
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: skippedCtx, TokenGroup: "group-a", ModelName: "shared-model", Retry: common.GetPointer(0),
	})
	require.NoError(t, err)
	require.Equal(t, 2, channel.Id)
	require.Equal(t, "group-b", selectedGroup)

	otherTokenCtx := newContext(43, "")
	_, err = PrepareModelGroupCombination(otherTokenCtx, "shared-model")
	require.NoError(t, err)
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: otherTokenCtx, TokenGroup: "group-a", ModelName: "shared-model", Retry: common.GetPointer(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, channel.Id)
	require.Equal(t, "group-a", selectedGroup)

	status, err := ResetModelGroupCombinationCircuitBreaker(token, "group-a")
	require.NoError(t, err)
	require.Equal(t, GroupCombinationBreakerStatusHealthy, status.Status)
}
