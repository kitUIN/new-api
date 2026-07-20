package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/samber/hot"
)

const ruleAutoGroupRuntimeNamespace = "new-api:rule_auto_group_runtime:v1"

type RuleAutoGroupState struct {
	CandidateIndex int      `json:"candidate_index"`
	FailureCount   int      `json:"failure_count"`
	SlowTTFTCount  int      `json:"slow_ttft_count"`
	Candidates     []string `json:"candidates"`
	Mode           string   `json:"mode"`
	UpdatedAt      int64    `json:"updated_at"`
}

type RuleAutoGroupRuntimeContext struct {
	Selector      string
	Mode          string
	Candidates    []string
	CurrentIndex  int
	SelectedGroup string
	FailureCount  int
	SlowTTFTCount int
	StateKey      string
	TTL           time.Duration
	SessionScoped bool
	Switched      bool
	SwitchReason  string
}

type ruleAutoGroupStateCodec struct{}

func (ruleAutoGroupStateCodec) Encode(value RuleAutoGroupState) (string, error) {
	data, err := common.Marshal(value)
	return string(data), err
}

func (ruleAutoGroupStateCodec) Decode(raw string) (RuleAutoGroupState, error) {
	var value RuleAutoGroupState
	if err := common.Unmarshal([]byte(raw), &value); err != nil {
		return value, err
	}
	return value, nil
}

var (
	ruleAutoGroupRuntimeCacheOnce sync.Once
	ruleAutoGroupRuntimeCache     *cachex.HybridCache[RuleAutoGroupState]
	ruleAutoGroupRuntimeLocks     [64]sync.Mutex
)

func getRuleAutoGroupRuntimeCache() *cachex.HybridCache[RuleAutoGroupState] {
	ruleAutoGroupRuntimeCacheOnce.Do(func() {
		setting := operation_setting.GetChannelAffinitySetting()
		capacity := setting.MaxEntries
		if capacity <= 0 {
			capacity = 100_000
		}
		ttlSeconds := setting.AutoGroupResetSeconds
		if ttlSeconds <= 0 {
			ttlSeconds = setting.DefaultTTLSeconds
		}
		if ttlSeconds <= 0 {
			ttlSeconds = 3600
		}
		ruleAutoGroupRuntimeCache = cachex.NewHybridCache[RuleAutoGroupState](cachex.HybridCacheConfig[RuleAutoGroupState]{
			Namespace:    cachex.Namespace(ruleAutoGroupRuntimeNamespace),
			Redis:        common.RDB,
			RedisCodec:   ruleAutoGroupStateCodec{},
			RedisEnabled: func() bool { return common.RedisEnabled && common.RDB != nil },
			Memory: func() *hot.HotCache[string, RuleAutoGroupState] {
				return hot.NewHotCache[string, RuleAutoGroupState](hot.LRU, capacity).
					WithTTL(time.Duration(ttlSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return ruleAutoGroupRuntimeCache
}

func getRuleAutoGroupRuntimeLock(key string) *sync.Mutex {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	return &ruleAutoGroupRuntimeLocks[hasher.Sum32()%uint32(len(ruleAutoGroupRuntimeLocks))]
}

func normalizeRuleAutoGroupState(state RuleAutoGroupState, candidates []string, mode string) RuleAutoGroupState {
	if state.Mode != mode || !sameStringSlice(state.Candidates, candidates) {
		return RuleAutoGroupState{Candidates: append([]string(nil), candidates...), Mode: mode}
	}
	if state.CandidateIndex < 0 || state.CandidateIndex >= len(candidates) {
		state.CandidateIndex = 0
		state.FailureCount = 0
		state.SlowTTFTCount = 0
	}
	return state
}

func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func nextRuleAutoGroupState(state RuleAutoGroupState, mode string, success bool, breakerFailure bool, ttftMs int64, hasTTFT bool) (RuleAutoGroupState, bool, string) {
	switched := false
	reason := ""
	if success {
		state.FailureCount = 0
		if mode == RuleAutoGroupModeBalanced && hasTTFT && ttftMs > 10_000 {
			state.SlowTTFTCount++
		} else {
			state.SlowTTFTCount = 0
		}
		if state.SlowTTFTCount >= 2 && state.CandidateIndex+1 < len(state.Candidates) {
			state.CandidateIndex++
			state.SlowTTFTCount = 0
			switched = true
			reason = "slow_ttft"
		}
		return state, switched, reason
	}

	state.SlowTTFTCount = 0
	if !breakerFailure {
		state.FailureCount = 0
		return state, false, ""
	}
	state.FailureCount++
	if state.FailureCount >= 2 && state.CandidateIndex+1 < len(state.Candidates) {
		state.CandidateIndex++
		state.FailureCount = 0
		switched = true
		reason = "consecutive_failures"
	}
	return state, switched, reason
}

func ruleAutoGroupRuntimeScope(c *gin.Context, selector, modelName string) (string, time.Duration, bool) {
	setting := operation_setting.GetChannelAffinitySetting()
	ttlSeconds := setting.AutoGroupResetSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = setting.DefaultTTLSeconds
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	sessionScoped := false
	scope := ""
	if meta, ok := getChannelAffinityMeta(c); ok && strings.TrimSpace(meta.CacheKeySuffix) != "" {
		scope = common.Sha1([]byte(meta.CacheKeySuffix))
		if meta.TTLSeconds > 0 {
			ttlSeconds = meta.TTLSeconds
		}
		sessionScoped = true
	}
	if scope == "" {
		scope = fmt.Sprintf("token:%d:%s", common.GetContextKeyInt(c, constant.ContextKeyTokenId), modelName)
	}
	key := fmt.Sprintf("%d:%s:%s:%s", common.GetContextKeyInt(c, constant.ContextKeyTokenId), selector, modelName, scope)
	return key, time.Duration(ttlSeconds) * time.Second, sessionScoped
}

func readRuleAutoGroupState(key string) (RuleAutoGroupState, bool, error) {
	value, found, err := getRuleAutoGroupRuntimeCache().Get(key)
	return value, found, err
}

func writeRuleAutoGroupState(key string, state RuleAutoGroupState, ttl time.Duration) error {
	state.UpdatedAt = common.GetTimestamp()
	return getRuleAutoGroupRuntimeCache().SetWithTTL(key, state, ttl)
}

func ApplyRuleAutoGroup(c *gin.Context, modelName string) bool {
	if c == nil {
		return false
	}
	selector := NormalizeRuleAutoGroupName(common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	if selector == "" {
		return false
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	candidates := GetRuleAutoGroupCandidates(userGroup, selector)
	mode, err := NormalizeRuleAutoGroupMode(common.GetContextKeyString(c, constant.ContextKeyTokenAutoGroupMode))
	if err != nil {
		mode = RuleAutoGroupModeLowRatio
	}
	stateKey, ttl, sessionScoped := ruleAutoGroupRuntimeScope(c, selector, modelName)
	lock := getRuleAutoGroupRuntimeLock(stateKey)
	lock.Lock()
	defer lock.Unlock()
	state, found, err := readRuleAutoGroupState(stateKey)
	if err != nil {
		common.SysLog("failed to read rule auto group state: " + err.Error())
	}
	if !found || err != nil {
		state = RuleAutoGroupState{Candidates: append([]string(nil), candidates...), Mode: mode}
	} else {
		state = normalizeRuleAutoGroupState(state, candidates, mode)
	}
	for state.CandidateIndex < len(candidates) && !model.HasAvailableChannelForGroupModel(candidates[state.CandidateIndex], modelName) {
		state.CandidateIndex++
		state.FailureCount = 0
		state.SlowTTFTCount = 0
	}
	if state.CandidateIndex >= len(candidates) {
		common.SetContextKey(c, constant.ContextKeyRuleAutoGroupRuntime, &RuleAutoGroupRuntimeContext{
			Selector:      selector,
			Mode:          mode,
			Candidates:    candidates,
			CurrentIndex:  len(candidates),
			StateKey:      stateKey,
			TTL:           ttl,
			SessionScoped: sessionScoped,
		})
		return true
	}
	state.Candidates = append([]string(nil), candidates...)
	state.Mode = mode
	_ = writeRuleAutoGroupState(stateKey, state, ttl)
	common.SetContextKey(c, constant.ContextKeyRuleAutoGroupRuntime, &RuleAutoGroupRuntimeContext{
		Selector:      selector,
		Mode:          mode,
		Candidates:    candidates,
		CurrentIndex:  state.CandidateIndex,
		SelectedGroup: candidates[state.CandidateIndex],
		FailureCount:  state.FailureCount,
		SlowTTFTCount: state.SlowTTFTCount,
		StateKey:      stateKey,
		TTL:           ttl,
		SessionScoped: sessionScoped,
	})
	common.SetContextKey(c, constant.ContextKeyAutoGroup, candidates[state.CandidateIndex])
	common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, state.CandidateIndex)
	return true
}

func GetRuleAutoGroupRuntime(c *gin.Context) (*RuleAutoGroupRuntimeContext, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := common.GetContextKeyType[*RuleAutoGroupRuntimeContext](c, constant.ContextKeyRuleAutoGroupRuntime)
	return value, ok && value != nil
}

func RecordRuleAutoGroupResult(c *gin.Context, success bool, breakerFailure bool, ttftMs int64, hasTTFT bool) bool {
	info, ok := GetRuleAutoGroupRuntime(c)
	if !ok || info.StateKey == "" || len(info.Candidates) == 0 || info.CurrentIndex >= len(info.Candidates) {
		return false
	}
	lock := getRuleAutoGroupRuntimeLock(info.StateKey)
	lock.Lock()
	defer lock.Unlock()
	if common.RedisEnabled && common.RDB != nil {
		var resultState RuleAutoGroupState
		var switched bool
		var switchReason string
		err := updateRuleAutoGroupStateWithWatch(context.Background(), info.StateKey, func(state RuleAutoGroupState) (RuleAutoGroupState, error) {
			if len(state.Candidates) == 0 {
				state = RuleAutoGroupState{Candidates: append([]string(nil), info.Candidates...), Mode: info.Mode, CandidateIndex: info.CurrentIndex}
			}
			state = normalizeRuleAutoGroupState(state, info.Candidates, info.Mode)
			if state.CandidateIndex > info.CurrentIndex {
				resultState = state
				return state, nil
			}
			if state.CandidateIndex < info.CurrentIndex {
				state.CandidateIndex = info.CurrentIndex
			}
			state, switched, switchReason = nextRuleAutoGroupState(state, info.Mode, success, breakerFailure, ttftMs, hasTTFT)
			resultState = state
			return state, nil
		}, info.TTL)
		if err != nil {
			common.SysLog("failed to atomically update rule auto group state: " + err.Error())
			return false
		}
		info.CurrentIndex = resultState.CandidateIndex
		info.SelectedGroup = ""
		if resultState.CandidateIndex < len(resultState.Candidates) {
			info.SelectedGroup = resultState.Candidates[resultState.CandidateIndex]
		}
		info.FailureCount = resultState.FailureCount
		info.SlowTTFTCount = resultState.SlowTTFTCount
		info.Switched = switched
		info.SwitchReason = switchReason
		common.SetContextKey(c, constant.ContextKeyRuleAutoGroupRuntime, info)
		if switched {
			common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, resultState.CandidateIndex)
			if info.SelectedGroup != "" {
				common.SetContextKey(c, constant.ContextKeyAutoGroup, info.SelectedGroup)
			}
		}
		return switched
	}
	state, found, err := readRuleAutoGroupState(info.StateKey)
	if err != nil || !found {
		state = RuleAutoGroupState{Candidates: append([]string(nil), info.Candidates...), Mode: info.Mode, CandidateIndex: info.CurrentIndex}
	}
	state = normalizeRuleAutoGroupState(state, info.Candidates, info.Mode)
	if state.CandidateIndex > info.CurrentIndex {
		return false
	}
	if state.CandidateIndex < info.CurrentIndex {
		state.CandidateIndex = info.CurrentIndex
	}
	state, switched, reason := nextRuleAutoGroupState(state, info.Mode, success, breakerFailure, ttftMs, hasTTFT)
	if err := writeRuleAutoGroupState(info.StateKey, state, info.TTL); err != nil {
		common.SysLog("failed to write rule auto group state: " + err.Error())
		return false
	}
	info.CurrentIndex = state.CandidateIndex
	info.SelectedGroup = ""
	if state.CandidateIndex < len(state.Candidates) {
		info.SelectedGroup = state.Candidates[state.CandidateIndex]
	}
	info.FailureCount = state.FailureCount
	info.SlowTTFTCount = state.SlowTTFTCount
	info.Switched = switched
	info.SwitchReason = reason
	common.SetContextKey(c, constant.ContextKeyRuleAutoGroupRuntime, info)
	if switched {
		common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, state.CandidateIndex)
		if info.SelectedGroup != "" {
			common.SetContextKey(c, constant.ContextKeyAutoGroup, info.SelectedGroup)
		}
	}
	return switched
}

func AppendRuleAutoGroupAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	info, ok := GetRuleAutoGroupRuntime(c)
	if !ok || adminInfo == nil {
		return
	}
	adminInfo["rule_auto_group"] = map[string]interface{}{
		"selector":        info.Selector,
		"mode":            info.Mode,
		"candidates":      info.Candidates,
		"current_index":   info.CurrentIndex,
		"selected_group":  info.SelectedGroup,
		"failure_count":   info.FailureCount,
		"slow_ttft_count": info.SlowTTFTCount,
		"session_scoped":  info.SessionScoped,
		"switched":        info.Switched,
		"switch_reason":   info.SwitchReason,
	}
}

func ruleAutoGroupStateRedisKey(key string) string {
	return getRuleAutoGroupRuntimeCache().FullKey(key)
}

func updateRuleAutoGroupStateWithWatch(ctx context.Context, key string, fn func(RuleAutoGroupState) (RuleAutoGroupState, error), ttl time.Duration) error {
	if !common.RedisEnabled || common.RDB == nil {
		return nil
	}
	redisKey := ruleAutoGroupStateRedisKey(key)
	for attempt := 0; attempt < 5; attempt++ {
		err := common.RDB.Watch(ctx, func(tx *redis.Tx) error {
			var state RuleAutoGroupState
			raw, err := tx.Get(ctx, redisKey).Result()
			if err != nil && err != redis.Nil {
				return err
			}
			if err == nil {
				decoded, decodeErr := (ruleAutoGroupStateCodec{}).Decode(raw)
				if decodeErr != nil {
					return decodeErr
				}
				state = decoded
			}
			next, err := fn(state)
			if err != nil {
				return err
			}
			next.UpdatedAt = common.GetTimestamp()
			encoded, err := (ruleAutoGroupStateCodec{}).Encode(next)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, redisKey, encoded, ttl)
				return nil
			})
			return err
		}, redisKey)
		if err == redis.TxFailedErr {
			continue
		}
		return err
	}
	return redis.TxFailedErr
}
