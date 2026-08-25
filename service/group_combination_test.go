package service

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveGroupCombinationChannel(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})

	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	highPriority := int64(10)
	lowPriority := int64(5)
	channels := []model.Channel{
		{Id: 2, Name: "luna-primary", Key: "sk-luna-primary", Status: common.ChannelStatusEnabled, Models: "gpt-5.6-luna", Group: "cheap", Priority: &highPriority},
		{Id: 3, Name: "luna-fallback", Key: "sk-luna-fallback", Status: common.ChannelStatusEnabled, Models: "gpt-5.6-luna", Group: "cheap", Priority: &lowPriority},
		{Id: 4, Name: "sol-cheap", Key: "sk-sol-cheap", Status: common.ChannelStatusEnabled, Models: "gpt-5.6-sol", Group: "cheap", Priority: &highPriority},
		{Id: 7, Name: "sol", Key: "sk-sol", Status: common.ChannelStatusEnabled, Models: "gpt-5.6-sol", Group: "premium", Priority: &highPriority},
		{Id: 9, Name: "luna-premium", Key: "sk-luna-premium", Status: common.ChannelStatusEnabled, Models: "gpt-5.6-luna", Group: "premium", Priority: &highPriority},
	}
	for i := range channels {
		require.NoError(t, db.Create(&channels[i]).Error)
		require.NoError(t, channels[i].AddAbilities(nil))
	}
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"codex":[{"group":"cheap","models":["gpt-5.6-luna"]},{"group":"premium","models":["gpt-5.6-luna","gpt-5.6-sol"]}]}`,
	))

	channel, selectedGroup, enabled, err := ResolveGroupCombinationChannel("codex", "gpt-5.6-sol")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "premium", selectedGroup)
	require.Equal(t, 7, channel.Id)

	_, _, enabled, err = ResolveGroupCombinationChannel("codex", "missing")
	require.True(t, enabled)
	require.ErrorContains(t, err, "成员分组均没有")

	channel, selectedGroup, enabled, err = ResolveGroupCombinationChannel("codex", "gpt-5.6-luna")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "cheap", selectedGroup)
	require.Equal(t, 2, channel.Id)

	channel, selectedGroup, enabled, err = ResolveGroupCombinationChannelWithExclusions("codex", "gpt-5.6-luna", []int{2})
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "cheap", selectedGroup)
	require.Equal(t, 3, channel.Id)

	channel, selectedGroup, enabled, err = ResolveGroupCombinationChannelWithExclusions("codex", "gpt-5.6-luna", []int{2, 3})
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "premium", selectedGroup)
	require.Equal(t, 9, channel.Id)

	selectedGroup, found := ResolveGroupCombinationChannelGroup("codex", "gpt-5.6-luna", 9)
	require.True(t, found)
	require.Equal(t, "premium", selectedGroup)

	_, found = ResolveGroupCombinationChannelGroup("codex", "gpt-5.6-sol", 4)
	require.False(t, found)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	retryParam := &RetryParam{
		Ctx:        ctx,
		TokenGroup: "codex",
		ModelName:  "gpt-5.6-luna",
		Retry:      common.GetPointer(0),
	}
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(retryParam)
	require.NoError(t, err)
	require.Equal(t, "cheap", selectedGroup)
	require.Equal(t, "cheap", common.GetContextKeyString(ctx, constant.ContextKeyUsingGroup))
	require.Equal(t, 2, channel.Id)

	retryParam.ExcludeChannel(2)
	retryParam.ExcludeChannel(3)
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(retryParam)
	require.NoError(t, err)
	require.Equal(t, "premium", selectedGroup)
	require.Equal(t, "premium", common.GetContextKeyString(ctx, constant.ContextKeyUsingGroup))
	require.Equal(t, 9, channel.Id)

	channel, selectedGroup, enabled, err = ResolveGroupCombinationChannel("default", "gpt-5.6-sol")
	require.NoError(t, err)
	require.False(t, enabled)
	require.Empty(t, selectedGroup)
	require.Nil(t, channel)

	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"cheap":[{"group":"premium","models":["gpt-5.6-luna"]},{"group":"cheap","models":["gpt-5.6-luna"]}]}`,
	))

	// Models without an explicit override keep using the original group.
	channel, selectedGroup, enabled, err = ResolveGroupCombinationChannel("cheap", "gpt-5.6-sol")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "cheap", selectedGroup)
	require.Equal(t, 4, channel.Id)
	require.True(t, model.IsGroupCombinationModelAvailable("cheap", "gpt-5.6-sol"))
	require.Contains(t, model.GetGroupCombinationEnabledModels("cheap"), "gpt-5.6-sol")

	// A retryable failure advances to the original group and the same session
	// starts from that downgraded member on subsequent requests.
	sessionSuffix := fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	setChannelAffinityContext(ctx, channelAffinityMeta{
		CacheKeySuffix: sessionSuffix,
		TTLSeconds:     60,
	})
	retryParam = &RetryParam{
		Ctx:        ctx,
		TokenGroup: "cheap",
		ModelName:  "gpt-5.6-luna",
		Retry:      common.GetPointer(0),
	}
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(retryParam)
	require.NoError(t, err)
	require.Equal(t, "premium", selectedGroup)
	require.Equal(t, 9, channel.Id)
	require.True(t, PrepareGroupCombinationFailover(ctx, retryParam))

	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(retryParam)
	require.NoError(t, err)
	require.Equal(t, "cheap", selectedGroup)
	require.Equal(t, 2, channel.Id)

	runtime, ok := ctx.Get(ginKeyGroupCombinationRuntime)
	require.True(t, ok)
	cacheKey := runtime.(*groupCombinationRuntime).CacheKey
	t.Cleanup(func() {
		_, _ = getGroupCombinationSessionCache().DeleteMany([]string{cacheKey})
	})

	nextCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	setChannelAffinityContext(nextCtx, channelAffinityMeta{
		CacheKeySuffix: sessionSuffix,
		TTLSeconds:     60,
	})
	nextRetryParam := &RetryParam{
		Ctx:        nextCtx,
		TokenGroup: "cheap",
		ModelName:  "gpt-5.6-luna",
		Retry:      common.GetPointer(0),
	}
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(nextRetryParam)
	require.NoError(t, err)
	require.Equal(t, "cheap", selectedGroup)
	require.Equal(t, 2, channel.Id)

	otherSessionCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	setChannelAffinityContext(otherSessionCtx, channelAffinityMeta{
		CacheKeySuffix: sessionSuffix + "-other",
		TTLSeconds:     60,
	})
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        otherSessionCtx,
		TokenGroup: "cheap",
		ModelName:  "gpt-5.6-luna",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.Equal(t, "premium", selectedGroup)
	require.Equal(t, 9, channel.Id)

	// Legacy definitions retain their exact model-to-channel semantics.
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"legacy":{"gpt-5.6-sol":7}}`,
	))
	channel, selectedGroup, enabled, err = ResolveGroupCombinationChannel("legacy", "gpt-5.6-sol")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "legacy", selectedGroup)
	require.Equal(t, 7, channel.Id)
	require.True(t, model.IsGroupCombinationModelAvailable("legacy", "gpt-5.6-sol"))
	require.True(t, model.IsGroupCombinationChannelAvailable("legacy", "gpt-5.6-sol", 7))
	require.False(t, model.IsGroupCombinationChannelAvailable("legacy", "gpt-5.6-sol", 4))

	selectedGroup, found = ResolveGroupCombinationChannelGroup("legacy", "gpt-5.6-sol", 7)
	require.True(t, found)
	require.Equal(t, "legacy", selectedGroup)
	_, _, enabled, err = ResolveGroupCombinationChannelWithExclusions("legacy", "gpt-5.6-sol", []int{7})
	require.True(t, enabled)
	require.ErrorContains(t, err, "固定渠道 #7 已被排除")
}

func TestGroupCombinationRatioUsesMemberGroups(t *testing.T) {
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"combo":9,"cheap":0.2,"premium":0.5}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"premium":0.4}}`))
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(`{"combo":[{"group":"cheap","models":["luna"]},{"group":"premium","models":["luna","sol"]}]}`))

	ratio, ratioRange := getGroupCombinationRatio("vip", "combo")
	require.Equal(t, "0.2x~0.4x", ratio)
	require.Equal(t, 0.2, ratioRange.Min)
	require.Equal(t, 0.4, ratioRange.Max)

	ratio, ratioRange = getGroupCombinationRatio("vip", "cheap")
	require.Equal(t, 0.2, ratio)
	require.Nil(t, ratioRange)
}
