package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const apiKeyGroupFailoverNamespace = "new-api:api_key_group_failover:v1"
const apiKeyGroupFailoverUnavailableGroup = "__api_key_group_failover_unavailable__"
const apiKeyGroupFailoverIdleTimeout = 2 * time.Hour

type SessionGroupFailoverState struct {
	LevelIndex    int      `json:"level_index"`
	FailureCount  int      `json:"failure_count"`
	Groups        []string `json:"groups"`
	UpdatedAt     int64    `json:"updated_at"`
	LastRequestAt int64    `json:"last_request_at,omitempty"`
	Generation    string   `json:"generation,omitempty"`
}

type SessionGroupFailoverContext struct {
	Enabled       bool     `json:"enabled"`
	Groups        []string `json:"groups"`
	CurrentLevel  int      `json:"current_level"`
	SelectedGroup string   `json:"selected_group"`
	FailureCount  int      `json:"failure_count"`
	Threshold     int      `json:"threshold"`
	Switched      bool     `json:"switched"`
	Scope         string   `json:"scope"`
	RedisKey      string   `json:"-"`
	Generation    string   `json:"-"`
}

type ApiKeyGroupFailoverRuntime struct {
	Enabled       bool     `json:"enabled"`
	Groups        []string `json:"groups"`
	CurrentLevel  int      `json:"current_level"`
	SelectedGroup string   `json:"selected_group"`
	FailureCount  int      `json:"failure_count"`
	Threshold     int      `json:"threshold"`
	Scope         string   `json:"scope"`
	UpdatedAt     int64    `json:"updated_at"`
}

func parseSessionFailoverGroups(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var groups []string
	if err := common.UnmarshalJsonStr(raw, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func encodeSessionFailoverGroups(groups []string) (string, error) {
	data, err := common.Marshal(groups)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeSessionFailoverGroupNames(groups []string, userGroup string) ([]string, error) {
	normalized := make([]string, 0, len(groups))
	seen := map[string]struct{}{}

	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			group = strings.TrimSpace(userGroup)
		}
		if group == "" {
			return nil, errors.New("故障转移分组不能为空")
		}
		if group == "auto" {
			return nil, errors.New("API Key 故障转移分组链不支持 auto")
		}
		if _, ok := seen[group]; ok {
			return nil, fmt.Errorf("故障转移分组重复: %s", group)
		}
		seen[group] = struct{}{}
		normalized = append(normalized, group)
	}
	return normalized, nil
}

func normalizeSessionFailoverGroups(groups []string, userGroup string) ([]string, error) {
	normalized, err := normalizeSessionFailoverGroupNames(groups, userGroup)
	if err != nil {
		return nil, err
	}
	usableGroups := GetUserUsableGroups(userGroup)
	for _, group := range normalized {
		if _, ok := usableGroups[group]; !ok {
			return nil, fmt.Errorf("无权访问 %s 分组", group)
		}
		if !ratio_setting.ContainsGroupRatio(group) {
			return nil, fmt.Errorf("分组 %s 已被弃用", group)
		}
	}
	return normalized, nil
}

func getRuntimeSessionFailoverUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup == "" {
		return groupsCopy
	}
	if specialSettings, ok := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup); ok {
		for specialGroup, desc := range specialSettings {
			if strings.HasPrefix(specialGroup, "-:") {
				delete(groupsCopy, strings.TrimPrefix(specialGroup, "-:"))
			} else if strings.HasPrefix(specialGroup, "+:") {
				groupsCopy[strings.TrimPrefix(specialGroup, "+:")] = desc
			} else {
				groupsCopy[specialGroup] = desc
			}
		}
	}
	return groupsCopy
}

func isSessionFailoverGroupRuntimeAvailable(group string, userGroup string, modelName string) bool {
	if group == "" {
		return false
	}
	if _, ok := getRuntimeSessionFailoverUsableGroups(userGroup)[group]; !ok {
		return false
	}
	if !ratio_setting.ContainsGroupRatio(group) {
		return false
	}
	if strings.TrimSpace(modelName) == "" {
		return true
	}
	return model.HasAvailableChannelForGroupModel(group, modelName)
}

func selectAvailableSessionFailoverLevel(groups []string, currentLevel int, userGroup string, modelName string) (int, bool) {
	if len(groups) == 0 {
		return 0, false
	}
	if currentLevel < 0 || currentLevel >= len(groups) {
		currentLevel = 0
	}
	for i := currentLevel; i < len(groups); i++ {
		if isSessionFailoverGroupRuntimeAvailable(groups[i], userGroup, modelName) {
			return i, true
		}
	}
	return currentLevel, false
}

func NormalizeTokenSessionFailover(token *model.Token, userGroup string) error {
	if token == nil {
		return errors.New("token is nil")
	}
	if token.SessionFailoverThreshold <= 0 {
		token.SessionFailoverThreshold = 3
	}
	if !token.SessionGroupFailoverEnabled {
		return nil
	}
	if token.SessionFailoverThreshold < 1 {
		return errors.New("API Key 故障转移连续失败次数至少为 1")
	}
	groups, err := parseSessionFailoverGroups(token.SessionFailoverGroups)
	if err != nil {
		return fmt.Errorf("API Key 故障转移分组格式错误: %w", err)
	}
	groups, err = normalizeSessionFailoverGroups(groups, userGroup)
	if err != nil {
		return err
	}
	if len(groups) < 2 {
		return errors.New("API Key 故障转移至少需要 2 个分组")
	}
	encoded, err := encodeSessionFailoverGroups(groups)
	if err != nil {
		return err
	}
	token.SessionFailoverGroups = encoded
	token.Group = groups[0]
	token.CrossGroupRetry = false
	return nil
}

func apiKeyGroupFailoverRedisKey(tokenID int) string {
	return fmt.Sprintf("%s:%d", apiKeyGroupFailoverNamespace, tokenID)
}

func ResetApiKeyGroupFailoverState(tokenID int) error {
	if tokenID <= 0 || !common.RedisEnabled || common.RDB == nil {
		return nil
	}
	return common.RedisDel(apiKeyGroupFailoverRedisKey(tokenID))
}

func sameFailoverGroups(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func readSessionGroupFailoverState(redisKey string) (SessionGroupFailoverState, bool) {
	if redisKey == "" || !common.RedisEnabled || common.RDB == nil {
		return SessionGroupFailoverState{}, false
	}
	raw, err := common.RedisGet(redisKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return SessionGroupFailoverState{}, false
	}
	return decodeSessionGroupFailoverState(raw)
}

func decodeSessionGroupFailoverState(raw string) (SessionGroupFailoverState, bool) {
	var state SessionGroupFailoverState
	if err := common.UnmarshalJsonStr(raw, &state); err != nil {
		common.SysLog("failed to unmarshal session group failover state: " + err.Error())
		return SessionGroupFailoverState{}, false
	}
	// Older states had no TTL or request timestamp. Use their last update once
	// so an idle key does not remain on a fallback group after upgrading.
	if state.LastRequestAt == 0 {
		state.LastRequestAt = state.UpdatedAt
	}
	if time.Until(time.Unix(state.LastRequestAt, 0).Add(apiKeyGroupFailoverIdleTimeout)) <= 0 {
		return SessionGroupFailoverState{}, false
	}
	return state, true
}

// updateSessionGroupFailoverState serializes request touches and result updates.
// Only request touches move LastRequestAt; results retain the remaining TTL.
func updateSessionGroupFailoverState(redisKey string, update func(SessionGroupFailoverState, bool) (SessionGroupFailoverState, bool)) (SessionGroupFailoverState, bool, error) {
	if redisKey == "" || !common.RedisEnabled || common.RDB == nil {
		return SessionGroupFailoverState{}, false, nil
	}
	ctx := context.Background()
	var result SessionGroupFailoverState
	var written bool
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		err = common.RDB.Watch(ctx, func(tx *redis.Tx) error {
			written = false
			raw, err := tx.Get(ctx, redisKey).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			var state SessionGroupFailoverState
			var found bool
			if err == nil {
				state, found = decodeSessionGroupFailoverState(raw)
			}
			next, shouldWrite := update(state, found)
			if !shouldWrite {
				return nil
			}
			ttl := time.Until(time.Unix(next.LastRequestAt, 0).Add(apiKeyGroupFailoverIdleTimeout))
			if ttl <= 0 {
				return nil
			}
			data, err := common.Marshal(next)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, redisKey, string(data), ttl)
				return nil
			})
			if err == nil {
				result, written = next, true
			}
			return err
		}, redisKey)
		if !errors.Is(err, redis.TxFailedErr) {
			break
		}
	}
	return result, written, err
}

// TouchApiKeyGroupFailoverState counts authenticated API key requests, including
// requests rejected later by routing. Dashboard reads never call this function.
func TouchApiKeyGroupFailoverState(token *model.Token) error {
	if token == nil || token.Id <= 0 || !token.SessionGroupFailoverEnabled || !common.RedisEnabled || common.RDB == nil {
		return nil
	}
	groups, err := parseSessionFailoverGroups(token.SessionFailoverGroups)
	if err != nil {
		return err
	}
	if len(groups) < 2 {
		return nil
	}
	_, _, err = updateSessionGroupFailoverState(apiKeyGroupFailoverRedisKey(token.Id), func(state SessionGroupFailoverState, found bool) (SessionGroupFailoverState, bool) {
		if !found || !sameFailoverGroups(state.Groups, groups) {
			state = SessionGroupFailoverState{Groups: groups, UpdatedAt: common.GetTimestamp()}
		}
		if state.Generation == "" {
			state.Generation = common.GetUUID()
		}
		state.LastRequestAt = common.GetTimestamp()
		return state, true
	})
	return err
}

func GetApiKeyGroupFailoverRuntime(token *model.Token) *ApiKeyGroupFailoverRuntime {
	if token == nil || !token.SessionGroupFailoverEnabled {
		return nil
	}
	groups, err := parseSessionFailoverGroups(token.SessionFailoverGroups)
	if err != nil || len(groups) < 2 {
		return nil
	}
	threshold := token.SessionFailoverThreshold
	if threshold < 1 {
		threshold = 3
	}
	level := 0
	failureCount := 0
	updatedAt := int64(0)
	if state, found := readSessionGroupFailoverState(apiKeyGroupFailoverRedisKey(token.Id)); found && sameFailoverGroups(state.Groups, groups) {
		level = state.LevelIndex
		failureCount = state.FailureCount
		updatedAt = state.UpdatedAt
	}
	if level < 0 || level >= len(groups) {
		level = 0
		failureCount = 0
	}
	return &ApiKeyGroupFailoverRuntime{
		Enabled:       true,
		Groups:        groups,
		CurrentLevel:  level,
		SelectedGroup: groups[level],
		FailureCount:  failureCount,
		Threshold:     threshold,
		Scope:         "api_key",
		UpdatedAt:     updatedAt,
	}
}

func getSessionGroupFailoverContext(c *gin.Context) (*SessionGroupFailoverContext, bool) {
	if c == nil {
		return nil, false
	}
	info, ok := common.GetContextKeyType[*SessionGroupFailoverContext](c, constant.ContextKeySessionGroupFailover)
	if !ok || info == nil || !info.Enabled {
		return nil, false
	}
	return info, true
}

func ApplySessionGroupFailover(c *gin.Context, modelName string) {
	if c == nil || !common.GetContextKeyBool(c, constant.ContextKeyTokenSessionGroupFailoverEnabled) {
		return
	}
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	rawGroups := common.GetContextKeyString(c, constant.ContextKeyTokenSessionFailoverGroups)
	groups, err := parseSessionFailoverGroups(rawGroups)
	if err != nil {
		common.SysLog("api key group failover groups parse failed: " + err.Error())
		return
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	groups, err = normalizeSessionFailoverGroupNames(groups, userGroup)
	if err != nil || len(groups) < 2 {
		if err != nil {
			common.SysLog("api key group failover groups invalid: " + err.Error())
		}
		return
	}
	threshold := common.GetContextKeyInt(c, constant.ContextKeyTokenSessionFailoverThreshold)
	if threshold < 1 {
		threshold = 3
	}
	tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
	if tokenID <= 0 {
		return
	}
	redisKey := apiKeyGroupFailoverRedisKey(tokenID)
	state, found := readSessionGroupFailoverState(redisKey)
	if found && !sameFailoverGroups(state.Groups, groups) {
		found = false
	}
	level := 0
	failureCount := 0
	if found {
		level = state.LevelIndex
		failureCount = state.FailureCount
	}
	if level < 0 || level >= len(groups) {
		level = 0
		failureCount = 0
	}
	selectedLevel, available := selectAvailableSessionFailoverLevel(groups, level, userGroup, modelName)
	switched := selectedLevel != level
	if !available {
		common.SysLog(fmt.Sprintf("api key group failover has no available group for model %s", modelName))
		common.SetContextKey(c, constant.ContextKeyUsingGroup, apiKeyGroupFailoverUnavailableGroup)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, apiKeyGroupFailoverUnavailableGroup)
		common.SetContextKey(c, constant.ContextKeySessionGroupFailover, &SessionGroupFailoverContext{
			Enabled:       true,
			Groups:        groups,
			CurrentLevel:  level,
			SelectedGroup: "",
			FailureCount:  failureCount,
			Threshold:     threshold,
			Scope:         "api_key",
			RedisKey:      redisKey,
			Generation:    state.Generation,
		})
		return
	}
	if switched {
		level = selectedLevel
		failureCount = 0
		_, _, err := updateSessionGroupFailoverState(redisKey, func(current SessionGroupFailoverState, found bool) (SessionGroupFailoverState, bool) {
			if !found || current.Generation != state.Generation || current.LevelIndex > level {
				return current, false
			}
			current.LevelIndex = level
			current.FailureCount = 0
			current.UpdatedAt = common.GetTimestamp()
			return current, true
		})
		if err != nil {
			common.SysLog("failed to advance api key group failover state after unavailable group: " + err.Error())
		}
		common.SysLog(fmt.Sprintf("api key group failover skipped unavailable group and selected P%d(%s) for model %s", level, groups[level], modelName))
	}
	selectedGroup := groups[level]
	common.SetContextKey(c, constant.ContextKeyUsingGroup, selectedGroup)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, selectedGroup)
	common.SetContextKey(c, constant.ContextKeySessionGroupFailover, &SessionGroupFailoverContext{
		Enabled:       true,
		Groups:        groups,
		CurrentLevel:  level,
		SelectedGroup: selectedGroup,
		FailureCount:  failureCount,
		Threshold:     threshold,
		Switched:      switched,
		Scope:         "api_key",
		RedisKey:      redisKey,
		Generation:    state.Generation,
	})
}

func RecordSessionGroupFailoverResult(c *gin.Context, success bool) {
	info, ok := getSessionGroupFailoverContext(c)
	if !ok || info.RedisKey == "" {
		return
	}
	var switched bool
	state, written, err := updateSessionGroupFailoverState(info.RedisKey, func(state SessionGroupFailoverState, found bool) (SessionGroupFailoverState, bool) {
		// A response from before an idle/manual reset must not restore old state.
		if !found || state.Generation != info.Generation || !sameFailoverGroups(state.Groups, info.Groups) {
			return state, false
		}
		if state.LevelIndex < 0 || state.LevelIndex >= len(info.Groups) {
			state.LevelIndex = info.CurrentLevel
			state.FailureCount = info.FailureCount
		}
		state, switched = nextSessionGroupFailoverState(state, *info, success)
		state.UpdatedAt = common.GetTimestamp()
		return state, true
	})
	if err != nil {
		common.SysLog("failed to write api key group failover state: " + err.Error())
		return
	}
	if !written {
		return
	}
	info.Switched = switched
	info.CurrentLevel = state.LevelIndex
	info.SelectedGroup = info.Groups[state.LevelIndex]
	info.FailureCount = state.FailureCount
	common.SetContextKey(c, constant.ContextKeySessionGroupFailover, info)
}

func nextSessionGroupFailoverState(state SessionGroupFailoverState, info SessionGroupFailoverContext, success bool) (SessionGroupFailoverState, bool) {
	switched := false
	if success {
		if state.LevelIndex <= info.CurrentLevel {
			state.LevelIndex = info.CurrentLevel
			state.FailureCount = 0
		}
	} else if state.LevelIndex <= info.CurrentLevel {
		state.LevelIndex = info.CurrentLevel
		state.FailureCount++
		if state.FailureCount >= info.Threshold && state.LevelIndex+1 < len(info.Groups) {
			state.LevelIndex++
			state.FailureCount = 0
			switched = true
		}
	}
	state.Groups = info.Groups
	return state, switched
}

func AppendSessionGroupFailoverAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	info, ok := getSessionGroupFailoverContext(c)
	if !ok {
		return
	}
	adminInfo["api_key_group_failover"] = map[string]interface{}{
		"enabled":        info.Enabled,
		"groups":         info.Groups,
		"current_level":  info.CurrentLevel,
		"selected_group": info.SelectedGroup,
		"failure_count":  info.FailureCount,
		"threshold":      info.Threshold,
		"switched":       info.Switched,
		"scope":          info.Scope,
	}
}
