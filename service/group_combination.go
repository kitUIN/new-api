package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
)

const (
	ginKeyGroupCombinationRuntime         = "group_combination_runtime"
	groupCombinationSessionCacheNamespace = "new-api:group_combination_session:v1"
	groupCombinationSourceSetting         = "setting"
	groupCombinationSourceToken           = "token"
)

var (
	groupCombinationSessionCacheOnce sync.Once
	groupCombinationSessionCache     *cachex.HybridCache[string]
)

type groupCombinationRuntime struct {
	RootGroup       string
	ModelName       string
	Members         []ratio_setting.GroupCombinationMember
	CurrentIndex    int
	SelectedIndex   int
	CacheKey        string
	TTLSeconds      int
	Source          string
	BreakerScope    string
	ConfigSignature string
}

func getGroupCombinationSessionCache() *cachex.HybridCache[string] {
	groupCombinationSessionCacheOnce.Do(func() {
		setting := operation_setting.GetChannelAffinitySetting()
		capacity := 100_000
		defaultTTLSeconds := 3600
		if setting != nil {
			if setting.MaxEntries > 0 {
				capacity = setting.MaxEntries
			}
			if setting.DefaultTTLSeconds > 0 {
				defaultTTLSeconds = setting.DefaultTTLSeconds
			}
		}
		groupCombinationSessionCache = cachex.NewHybridCache[string](cachex.HybridCacheConfig[string]{
			Namespace: groupCombinationSessionCacheNamespace,
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.StringCodec{},
			Memory: func() *hot.HotCache[string, string] {
				return hot.NewHotCache[string, string](hot.LRU, capacity).
					WithTTL(time.Duration(defaultTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return groupCombinationSessionCache
}

func groupCombinationMembersForModel(group, modelName string, members []ratio_setting.GroupCombinationMember, fallbackToRoot bool) []ratio_setting.GroupCombinationMember {
	candidates := make([]ratio_setting.GroupCombinationMember, 0, len(members))
	for _, member := range members {
		if ratio_setting.GroupCombinationMemberSupportsModel(member, modelName) {
			candidates = append(candidates, member)
		}
	}
	if len(candidates) == 0 && fallbackToRoot {
		// Models without an explicit override retain the original group's behavior.
		candidates = append(candidates, ratio_setting.GroupCombinationMember{
			Group:  group,
			Models: []string{modelName},
		})
	}
	return candidates
}

func isGroupCombinationRuntimeMemberSkipped(source, scope, signature, group string) bool {
	if source == groupCombinationSourceToken {
		return isScopedGroupCombinationMemberSkipped(scope, group, signature)
	}
	return isGroupCombinationMemberSkipped(group)
}

func groupCombinationSessionCacheKey(c *gin.Context, group, modelName string, members []ratio_setting.GroupCombinationMember) (string, int, bool) {
	if c == nil {
		return "", 0, false
	}
	meta, ok := getChannelAffinityMeta(c)
	if !ok || strings.TrimSpace(meta.CacheKeySuffix) == "" {
		return "", 0, false
	}
	memberGroups := make([]string, 0, len(members))
	for _, member := range members {
		memberGroups = append(memberGroups, member.Group)
	}
	keyMaterial := strings.Join([]string{
		group,
		modelName,
		strings.Join(memberGroups, "\x00"),
		meta.CacheKeySuffix,
	}, "\x00")
	return common.Sha1([]byte(keyMaterial)), meta.TTLSeconds, true
}

func groupCombinationSessionState(c *gin.Context, rootGroup, modelName string, candidates []ratio_setting.GroupCombinationMember) (string, int, int) {
	cacheKey, ttlSeconds, ok := groupCombinationSessionCacheKey(c, rootGroup, modelName, candidates)
	if !ok {
		return "", 0, 0
	}
	currentIndex := 0
	selectedGroup, found, err := getGroupCombinationSessionCache().Get(cacheKey)
	if err != nil {
		common.SysError(fmt.Sprintf("group combination session cache get failed: err=%v", err))
	} else if found {
		for i, member := range candidates {
			if member.Group == selectedGroup {
				currentIndex = i
				break
			}
		}
	}
	return cacheKey, ttlSeconds, currentIndex
}

func getGroupCombinationRuntimeForCandidates(c *gin.Context, rootGroup, modelName string, candidates []ratio_setting.GroupCombinationMember, source, breakerScope, configSignature string) *groupCombinationRuntime {
	if c != nil {
		if existing, ok := c.Get(ginKeyGroupCombinationRuntime); ok {
			if runtime, ok := existing.(*groupCombinationRuntime); ok && runtime != nil && runtime.RootGroup == rootGroup && runtime.ModelName == modelName && runtime.Source == source {
				return runtime
			}
		}
	}

	availableCandidates := make([]ratio_setting.GroupCombinationMember, 0, len(candidates))
	for _, member := range candidates {
		if isGroupCombinationRuntimeMemberSkipped(source, breakerScope, configSignature, member.Group) {
			continue
		}
		availableCandidates = append(availableCandidates, member)
	}
	candidates = availableCandidates
	runtime := &groupCombinationRuntime{
		RootGroup:       rootGroup,
		ModelName:       modelName,
		Members:         candidates,
		CurrentIndex:    0,
		SelectedIndex:   -1,
		Source:          source,
		BreakerScope:    breakerScope,
		ConfigSignature: configSignature,
	}
	if cacheKey, ttlSeconds, currentIndex := groupCombinationSessionState(c, rootGroup, modelName, candidates); cacheKey != "" {
		runtime.CacheKey = cacheKey
		runtime.TTLSeconds = ttlSeconds
		runtime.CurrentIndex = currentIndex
	}
	if c != nil {
		c.Set(ginKeyGroupCombinationRuntime, runtime)
	}
	return runtime
}

func getGroupCombinationRuntime(c *gin.Context, group, modelName string, members []ratio_setting.GroupCombinationMember) *groupCombinationRuntime {
	candidates := groupCombinationMembersForModel(group, modelName, members, true)
	return getGroupCombinationRuntimeForCandidates(c, group, modelName, candidates, groupCombinationSourceSetting, "", "")
}

func persistGroupCombinationRuntime(runtime *groupCombinationRuntime) {
	if runtime == nil || runtime.CacheKey == "" || runtime.CurrentIndex < 0 || runtime.CurrentIndex >= len(runtime.Members) {
		return
	}
	ttlSeconds := runtime.TTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	if err := getGroupCombinationSessionCache().SetWithTTL(runtime.CacheKey, runtime.Members[runtime.CurrentIndex].Group, time.Duration(ttlSeconds)*time.Second); err != nil {
		common.SysError(fmt.Sprintf("group combination session cache set failed: err=%v", err))
	}
}

func resolveGroupCombinationRuntimeChannel(runtime *groupCombinationRuntime, combinationName, modelName string, excludedChannelIDs []int) (*model.Channel, string, error) {
	if runtime == nil {
		return nil, "", fmt.Errorf("%s 运行状态未初始化", combinationName)
	}
	for i := runtime.CurrentIndex; i < len(runtime.Members); i++ {
		member := runtime.Members[i]
		runtime.SelectedIndex = i
		channel, err := model.GetRandomSatisfiedChannelWithExclusions(member.Group, modelName, 0, excludedChannelIDs)
		if err != nil {
			return nil, member.Group, fmt.Errorf("%s 从成员分组 %s 选择渠道失败: %w", combinationName, member.Group, err)
		}
		if channel != nil {
			if i > runtime.CurrentIndex {
				runtime.CurrentIndex = i
				persistGroupCombinationRuntime(runtime)
			}
			return channel, member.Group, nil
		}
	}
	return nil, "", fmt.Errorf("%s 的成员分组均没有模型 %s 的可用渠道", combinationName, modelName)
}

// ResolveGroupCombinationChannel selects the first member group that has an
// available channel for the requested model.
func ResolveGroupCombinationChannel(group, modelName string) (*model.Channel, string, bool, error) {
	return ResolveGroupCombinationChannelWithExclusions(group, modelName, nil)
}

func ResolveGroupCombinationChannelWithExclusions(group, modelName string, excludedChannelIDs []int) (*model.Channel, string, bool, error) {
	return resolveGroupCombinationChannelWithContext(nil, group, modelName, excludedChannelIDs)
}

func resolveGroupCombinationChannelWithContext(c *gin.Context, group, modelName string, excludedChannelIDs []int) (*model.Channel, string, bool, error) {
	if channelID, configured, routed := ratio_setting.GetGroupCombinationChannelID(group, modelName); configured {
		if !routed {
			return nil, group, true, fmt.Errorf("组合分组 %s 未配置模型 %s 的渠道", group, modelName)
		}
		for _, excludedChannelID := range excludedChannelIDs {
			if excludedChannelID == channelID {
				return nil, group, true, fmt.Errorf("组合分组 %s 配置的固定渠道 #%d 已被排除", group, channelID)
			}
		}
		channel, err := model.CacheGetChannel(channelID)
		if err != nil || channel == nil {
			return nil, group, true, fmt.Errorf("组合分组 %s 配置的渠道 #%d 不存在", group, channelID)
		}
		if channel.Status != common.ChannelStatusEnabled {
			return nil, group, true, fmt.Errorf("组合分组 %s 配置的渠道 #%d 已停用", group, channelID)
		}
		if !model.ChannelExposesRequestedModel(channel, modelName) {
			return nil, group, true, fmt.Errorf("组合分组 %s 配置的渠道 #%d 不支持模型 %s", group, channelID, modelName)
		}
		return channel, group, true, nil
	}

	members, configured := ratio_setting.GetGroupCombinationMembers(group)
	if !configured {
		return nil, "", false, nil
	}

	runtime := getGroupCombinationRuntime(c, group, modelName, members)
	channel, selectedGroup, err := resolveGroupCombinationRuntimeChannel(runtime, "组合分组 "+group, modelName, excludedChannelIDs)
	return channel, selectedGroup, true, err
}

// PrepareGroupCombinationFailover advances a retryable request to the next
// configured member and persists that downgrade for the same model/session.
func PrepareGroupCombinationFailover(c *gin.Context, retryParam *RetryParam) bool {
	if c == nil || retryParam == nil {
		return false
	}
	anyRuntime, ok := c.Get(ginKeyGroupCombinationRuntime)
	if !ok {
		return false
	}
	runtime, ok := anyRuntime.(*groupCombinationRuntime)
	if !ok || runtime == nil || runtime.ModelName != retryParam.ModelName {
		return false
	}
	if runtime.Source == groupCombinationSourceSetting && runtime.RootGroup != retryParam.TokenGroup {
		return false
	}
	if runtime.Source == groupCombinationSourceToken && !common.GetContextKeyBool(c, constant.ContextKeyTokenModelGroupCombinationEnabled) {
		return false
	}
	nextIndex := runtime.SelectedIndex + 1
	if nextIndex <= runtime.CurrentIndex {
		nextIndex = runtime.CurrentIndex + 1
	}
	if nextIndex < 0 || nextIndex >= len(runtime.Members) {
		return false
	}
	if runtime.SelectedIndex >= 0 && runtime.SelectedIndex < len(runtime.Members) {
		recordGroupCombinationRuntimeMemberFailure(runtime, runtime.Members[runtime.SelectedIndex].Group)
	}
	runtime.CurrentIndex = nextIndex
	runtime.SelectedIndex = nextIndex
	persistGroupCombinationRuntime(runtime)
	retryParam.SetRetry(0)
	retryParam.ResetRetryNextTry()
	return true
}

func ResolveGroupCombinationChannelGroup(group, modelName string, channelID int) (string, bool) {
	if routedChannelID, configured, routed := ratio_setting.GetGroupCombinationChannelID(group, modelName); configured {
		if routed && routedChannelID == channelID && model.IsGroupCombinationChannelAvailable(group, modelName, channelID) {
			return group, true
		}
		return "", false
	}

	members, configured := ratio_setting.GetGroupCombinationMembers(group)
	if !configured {
		return "", false
	}
	configuredForModel := false
	for _, member := range members {
		if !ratio_setting.GroupCombinationMemberSupportsModel(member, modelName) {
			continue
		}
		configuredForModel = true
		if model.IsChannelEnabledForConcreteGroupModel(member.Group, modelName, channelID) {
			return member.Group, true
		}
	}
	if !configuredForModel && model.IsChannelEnabledForConcreteGroupModel(group, modelName, channelID) {
		return group, true
	}
	return "", false
}
