package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	GroupCombinationBreakerFailureThreshold = 5
	GroupCombinationBreakerCooldownSeconds  = 2 * 60 * 60

	groupCombinationBreakerNamespace = "new-api:group_combination_breaker:v1"
	groupCombinationBreakerRedisTTL  = time.Duration(GroupCombinationBreakerCooldownSeconds) * time.Second
)

const (
	GroupCombinationBreakerStatusHealthy = "healthy"
	GroupCombinationBreakerStatusWarning = "warning"
	GroupCombinationBreakerStatusSkipped = "skipped"
)

type GroupCombinationBreakerState struct {
	Group               string `json:"group"`
	ConfigSignature     string `json:"config_signature"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	SkippedUntil        int64  `json:"skipped_until"`
	UpdatedAt           int64  `json:"updated_at"`
}

type GroupCombinationBreakerStatus struct {
	Group               string `json:"group"`
	Status              string `json:"status"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	SkippedUntil        int64  `json:"skipped_until"`
	UpdatedAt           int64  `json:"updated_at"`
}

type GroupCombinationBreakerSummary struct {
	FailureThreshold int                             `json:"failure_threshold"`
	CooldownSeconds  int                             `json:"cooldown_seconds"`
	Groups           []GroupCombinationBreakerStatus `json:"groups"`
}

type groupCombinationBreakerCodec struct{}

func (groupCombinationBreakerCodec) Encode(state GroupCombinationBreakerState) (string, error) {
	data, err := common.Marshal(state)
	return string(data), err
}

func (groupCombinationBreakerCodec) Decode(raw string) (GroupCombinationBreakerState, error) {
	var state GroupCombinationBreakerState
	if err := common.Unmarshal([]byte(raw), &state); err != nil {
		return state, err
	}
	return state, nil
}

var (
	groupCombinationBreakerMemoryMu sync.Mutex
	groupCombinationBreakerMemory   = make(map[string]GroupCombinationBreakerState)
	groupCombinationBreakerNow      = time.Now
)

func groupCombinationBreakerRedisOn() bool {
	return common.RedisEnabled && common.RDB != nil
}

func groupCombinationBreakerStateKey(group string) string {
	return common.Sha1([]byte(strings.TrimSpace(group)))
}

func groupCombinationBreakerRedisKey(group string) string {
	namespace := cachex.Namespace(groupCombinationBreakerNamespace)
	return namespace.FullKey(groupCombinationBreakerStateKey(group))
}

func groupCombinationMemberConfigSignatures() map[string]string {
	recordsByGroup := make(map[string][]string)
	for rootGroup, members := range ratio_setting.GetGroupCombinationsCopy() {
		for index, member := range members {
			models := append([]string(nil), member.Models...)
			sort.Strings(models)
			recordsByGroup[member.Group] = append(recordsByGroup[member.Group], strings.Join([]string{
				rootGroup,
				strconv.Itoa(index),
				strings.Join(models, "\x00"),
			}, "\x00"))
		}
	}

	signatures := make(map[string]string, len(recordsByGroup))
	for group, records := range recordsByGroup {
		sort.Strings(records)
		signatures[group] = common.Sha1([]byte(strings.Join(records, "\x01")))
	}
	return signatures
}

func groupCombinationMemberConfigSignature(group string) (string, bool) {
	signature, ok := groupCombinationMemberConfigSignatures()[strings.TrimSpace(group)]
	return signature, ok
}

func IsGroupCombinationCircuitBreakerMember(group string) bool {
	_, ok := groupCombinationMemberConfigSignature(group)
	return ok
}

func normalizeGroupCombinationBreakerState(state GroupCombinationBreakerState, group, signature string, now int64) (GroupCombinationBreakerState, bool) {
	if state.Group != group || state.ConfigSignature != signature {
		return GroupCombinationBreakerState{}, false
	}
	if state.SkippedUntil > 0 && state.SkippedUntil <= now {
		return GroupCombinationBreakerState{}, false
	}
	if state.ConsecutiveFailures <= 0 {
		return GroupCombinationBreakerState{}, false
	}
	return state, true
}

func readGroupCombinationBreakerState(group, signature string) (GroupCombinationBreakerState, bool, error) {
	now := groupCombinationBreakerNow().Unix()
	key := groupCombinationBreakerStateKey(group)
	if groupCombinationBreakerRedisOn() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		raw, err := common.RDB.Get(ctx, groupCombinationBreakerRedisKey(group)).Result()
		if errors.Is(err, redis.Nil) {
			return GroupCombinationBreakerState{}, false, nil
		}
		if err != nil {
			return GroupCombinationBreakerState{}, false, err
		}
		state, err := (groupCombinationBreakerCodec{}).Decode(raw)
		if err != nil {
			return GroupCombinationBreakerState{}, false, err
		}
		state, valid := normalizeGroupCombinationBreakerState(state, group, signature, now)
		if !valid {
			if err := deleteGroupCombinationBreakerRedisStateIfUnchanged(ctx, group, raw); err != nil {
				return GroupCombinationBreakerState{}, false, err
			}
		}
		return state, valid, nil
	}

	groupCombinationBreakerMemoryMu.Lock()
	defer groupCombinationBreakerMemoryMu.Unlock()
	state, found := groupCombinationBreakerMemory[key]
	if !found {
		return GroupCombinationBreakerState{}, false, nil
	}
	state, valid := normalizeGroupCombinationBreakerState(state, group, signature, now)
	if !valid {
		delete(groupCombinationBreakerMemory, key)
		return GroupCombinationBreakerState{}, false, nil
	}
	return state, true, nil
}

func deleteGroupCombinationBreakerRedisStateIfUnchanged(ctx context.Context, group, expected string) error {
	redisKey := groupCombinationBreakerRedisKey(group)
	err := common.RDB.Watch(ctx, func(tx *redis.Tx) error {
		current, err := tx.Get(ctx, redisKey).Result()
		if errors.Is(err, redis.Nil) || (err == nil && current != expected) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, redisKey)
			return nil
		})
		return err
	}, redisKey)
	if errors.Is(err, redis.TxFailedErr) {
		// A newer writer replaced the stale value, so it must not be deleted.
		return nil
	}
	return err
}

func isGroupCombinationMemberSkipped(group string) bool {
	signature, ok := groupCombinationMemberConfigSignature(group)
	if !ok {
		return false
	}
	state, found, err := readGroupCombinationBreakerState(group, signature)
	if err != nil {
		common.SysError(fmt.Sprintf("group combination breaker read failed: group=%s err=%v", group, err))
		return false
	}
	return found && state.SkippedUntil > groupCombinationBreakerNow().Unix()
}

func updateGroupCombinationBreakerStateRedis(group, signature string, success bool) (GroupCombinationBreakerState, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	redisKey := groupCombinationBreakerRedisKey(group)
	codec := groupCombinationBreakerCodec{}

	var result GroupCombinationBreakerState
	var opened bool
	for attempt := 0; attempt < 5; attempt++ {
		opened = false
		err := common.RDB.Watch(ctx, func(tx *redis.Tx) error {
			now := groupCombinationBreakerNow().Unix()
			state := GroupCombinationBreakerState{}
			raw, err := tx.Get(ctx, redisKey).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			if err == nil {
				decoded, decodeErr := codec.Decode(raw)
				if decodeErr != nil {
					return decodeErr
				}
				state, _ = normalizeGroupCombinationBreakerState(decoded, group, signature, now)
			}

			if success {
				if state.SkippedUntil > now {
					result = state
					return nil
				}
				result = GroupCombinationBreakerState{}
				_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
					pipe.Del(ctx, redisKey)
					return nil
				})
				return err
			}

			if state.SkippedUntil > now {
				result = state
				return nil
			}
			state.Group = group
			state.ConfigSignature = signature
			state.ConsecutiveFailures++
			state.UpdatedAt = now
			ttl := time.Duration(0)
			if state.ConsecutiveFailures >= GroupCombinationBreakerFailureThreshold {
				state.ConsecutiveFailures = GroupCombinationBreakerFailureThreshold
				state.SkippedUntil = now + GroupCombinationBreakerCooldownSeconds
				ttl = groupCombinationBreakerRedisTTL
				opened = true
			}
			encoded, err := codec.Encode(state)
			if err != nil {
				return err
			}
			result = state
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, redisKey, encoded, ttl)
				return nil
			})
			return err
		}, redisKey)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		return result, opened, err
	}
	return GroupCombinationBreakerState{}, false, redis.TxFailedErr
}

func updateGroupCombinationBreakerStateMemory(group, signature string, success bool) (GroupCombinationBreakerState, bool) {
	groupCombinationBreakerMemoryMu.Lock()
	defer groupCombinationBreakerMemoryMu.Unlock()

	now := groupCombinationBreakerNow().Unix()
	key := groupCombinationBreakerStateKey(group)
	state, _ := normalizeGroupCombinationBreakerState(groupCombinationBreakerMemory[key], group, signature, now)
	if success {
		if state.SkippedUntil > now {
			return state, false
		}
		delete(groupCombinationBreakerMemory, key)
		return GroupCombinationBreakerState{}, false
	}
	if state.SkippedUntil > now {
		return state, false
	}

	state.Group = group
	state.ConfigSignature = signature
	state.ConsecutiveFailures++
	state.UpdatedAt = now
	opened := false
	if state.ConsecutiveFailures >= GroupCombinationBreakerFailureThreshold {
		state.ConsecutiveFailures = GroupCombinationBreakerFailureThreshold
		state.SkippedUntil = now + GroupCombinationBreakerCooldownSeconds
		opened = true
	}
	groupCombinationBreakerMemory[key] = state
	return state, opened
}

func updateGroupCombinationBreakerState(group string, success bool) (GroupCombinationBreakerState, bool, error) {
	group = strings.TrimSpace(group)
	signature, ok := groupCombinationMemberConfigSignature(group)
	if !ok {
		return GroupCombinationBreakerState{}, false, nil
	}
	if groupCombinationBreakerRedisOn() {
		return updateGroupCombinationBreakerStateRedis(group, signature, success)
	}
	state, opened := updateGroupCombinationBreakerStateMemory(group, signature, success)
	return state, opened, nil
}

func recordGroupCombinationMemberFailure(group string) {
	state, opened, err := updateGroupCombinationBreakerState(group, false)
	if err != nil {
		common.SysError(fmt.Sprintf("group combination breaker update failed: group=%s err=%v", group, err))
		return
	}
	if opened {
		common.SysLog(fmt.Sprintf("group combination member skipped after %d consecutive failures: group=%s skipped_until=%d", state.ConsecutiveFailures, group, state.SkippedUntil))
	}
}

func RecordGroupCombinationSuccess(c *gin.Context) {
	if c == nil {
		return
	}
	value, ok := c.Get(ginKeyGroupCombinationRuntime)
	if !ok {
		return
	}
	runtime, ok := value.(*groupCombinationRuntime)
	if !ok || runtime == nil || runtime.SelectedIndex < 0 || runtime.SelectedIndex >= len(runtime.Members) {
		return
	}
	group := runtime.Members[runtime.SelectedIndex].Group
	if _, _, err := updateGroupCombinationBreakerState(group, true); err != nil {
		common.SysError(fmt.Sprintf("group combination breaker success update failed: group=%s err=%v", group, err))
	}
}

func groupCombinationBreakerStatus(group, signature string) (GroupCombinationBreakerStatus, error) {
	status := GroupCombinationBreakerStatus{Group: group, Status: GroupCombinationBreakerStatusHealthy}
	state, found, err := readGroupCombinationBreakerState(group, signature)
	if err != nil {
		return status, err
	}
	if !found {
		return status, nil
	}
	status.ConsecutiveFailures = state.ConsecutiveFailures
	status.SkippedUntil = state.SkippedUntil
	status.UpdatedAt = state.UpdatedAt
	if state.SkippedUntil > groupCombinationBreakerNow().Unix() {
		status.Status = GroupCombinationBreakerStatusSkipped
	} else if state.ConsecutiveFailures > 0 {
		status.Status = GroupCombinationBreakerStatusWarning
	}
	return status, nil
}

func GetGroupCombinationCircuitBreakerSummary() (GroupCombinationBreakerSummary, error) {
	summary := GroupCombinationBreakerSummary{
		FailureThreshold: GroupCombinationBreakerFailureThreshold,
		CooldownSeconds:  GroupCombinationBreakerCooldownSeconds,
		Groups:           make([]GroupCombinationBreakerStatus, 0),
	}
	signatures := groupCombinationMemberConfigSignatures()
	groups := make([]string, 0, len(signatures))
	for group := range signatures {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	for _, group := range groups {
		status, err := groupCombinationBreakerStatus(group, signatures[group])
		if err != nil {
			return summary, err
		}
		summary.Groups = append(summary.Groups, status)
	}
	return summary, nil
}

func ResetGroupCombinationCircuitBreaker(group string) (GroupCombinationBreakerStatus, error) {
	group = strings.TrimSpace(group)
	signature, ok := groupCombinationMemberConfigSignature(group)
	if !ok {
		return GroupCombinationBreakerStatus{}, fmt.Errorf("分组 %s 不是当前组合模式成员", group)
	}
	if groupCombinationBreakerRedisOn() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := common.RDB.Del(ctx, groupCombinationBreakerRedisKey(group)).Err(); err != nil {
			return GroupCombinationBreakerStatus{}, err
		}
	} else {
		groupCombinationBreakerMemoryMu.Lock()
		delete(groupCombinationBreakerMemory, groupCombinationBreakerStateKey(group))
		groupCombinationBreakerMemoryMu.Unlock()
	}
	common.SysLog(fmt.Sprintf("group combination circuit breaker manually reset: group=%s", group))
	return groupCombinationBreakerStatus(group, signature)
}
