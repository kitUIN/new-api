package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const sessionGroupFailoverNamespace = "new-api:session_group_failover:v1"

type channelAffinitySession struct {
	RuleName       string
	KeyFingerprint string
	KeyHint        string
	TTLSeconds     int
}

type SessionGroupFailoverState struct {
	LevelIndex   int      `json:"level_index"`
	FailureCount int      `json:"failure_count"`
	Groups       []string `json:"groups"`
	UpdatedAt    int64    `json:"updated_at"`
}

type SessionGroupFailoverContext struct {
	Enabled        bool     `json:"enabled"`
	Groups         []string `json:"groups"`
	CurrentLevel   int      `json:"current_level"`
	SelectedGroup  string   `json:"selected_group"`
	FailureCount   int      `json:"failure_count"`
	Threshold      int      `json:"threshold"`
	Switched       bool     `json:"switched"`
	KeyFingerprint string   `json:"key_fp"`
	KeyHint        string   `json:"key_hint,omitempty"`
	RuleName       string   `json:"rule_name"`
	RedisKey       string   `json:"-"`
	TTLSeconds     int      `json:"-"`
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

func normalizeSessionFailoverGroups(groups []string, userGroup string) ([]string, error) {
	normalized := make([]string, 0, len(groups))
	seen := map[string]struct{}{}
	usableGroups := GetUserUsableGroups(userGroup)

	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			group = strings.TrimSpace(userGroup)
		}
		if group == "" {
			return nil, errors.New("故障转移分组不能为空")
		}
		if group == "auto" {
			return nil, errors.New("会话故障转移分组链不支持 auto")
		}
		if _, ok := seen[group]; ok {
			return nil, fmt.Errorf("故障转移分组重复: %s", group)
		}
		if _, ok := usableGroups[group]; !ok {
			return nil, fmt.Errorf("无权访问 %s 分组", group)
		}
		if !ratio_setting.ContainsGroupRatio(group) {
			return nil, fmt.Errorf("分组 %s 已被弃用", group)
		}
		seen[group] = struct{}{}
		normalized = append(normalized, group)
	}
	return normalized, nil
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
		return errors.New("会话故障转移连续失败次数至少为 1")
	}
	groups, err := parseSessionFailoverGroups(token.SessionFailoverGroups)
	if err != nil {
		return fmt.Errorf("会话故障转移分组格式错误: %w", err)
	}
	groups, err = normalizeSessionFailoverGroups(groups, userGroup)
	if err != nil {
		return err
	}
	if len(groups) < 2 {
		return errors.New("会话故障转移至少需要 2 个分组")
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

func resolveChannelAffinitySession(c *gin.Context, modelName string) (channelAffinitySession, bool) {
	if meta, ok := getChannelAffinityMeta(c); ok && strings.TrimSpace(meta.KeyFingerprint) != "" {
		ttlSeconds := meta.TTLSeconds
		if ttlSeconds <= 0 {
			ttlSeconds = 3600
		}
		return channelAffinitySession{
			RuleName:       strings.TrimSpace(meta.RuleName),
			KeyFingerprint: strings.TrimSpace(meta.KeyFingerprint),
			KeyHint:        strings.TrimSpace(meta.KeyHint),
			TTLSeconds:     ttlSeconds,
		}, true
	}

	setting := operation_setting.GetChannelAffinitySetting()
	if setting == nil {
		return channelAffinitySession{}, false
	}
	path := ""
	if c != nil && c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	userAgent := ""
	if c != nil && c.Request != nil {
		userAgent = c.Request.UserAgent()
	}
	for _, rule := range setting.Rules {
		if !matchAnyRegexCached(rule.ModelRegex, modelName) {
			continue
		}
		if len(rule.PathRegex) > 0 && !matchAnyRegexCached(rule.PathRegex, path) {
			continue
		}
		if len(rule.UserAgentInclude) > 0 && !matchAnyIncludeFold(rule.UserAgentInclude, userAgent) {
			continue
		}
		var affinityValue string
		for _, src := range rule.KeySources {
			affinityValue = extractChannelAffinityValue(c, src)
			if affinityValue != "" {
				break
			}
		}
		if affinityValue == "" {
			continue
		}
		if rule.ValueRegex != "" && !matchAnyRegexCached([]string{rule.ValueRegex}, affinityValue) {
			continue
		}
		ttlSeconds := rule.TTLSeconds
		if ttlSeconds <= 0 {
			ttlSeconds = setting.DefaultTTLSeconds
		}
		if ttlSeconds <= 0 {
			ttlSeconds = 3600
		}
		return channelAffinitySession{
			RuleName:       strings.TrimSpace(rule.Name),
			KeyFingerprint: affinityFingerprint(affinityValue),
			KeyHint:        buildChannelAffinityKeyHint(affinityValue),
			TTLSeconds:     ttlSeconds,
		}, true
	}
	return channelAffinitySession{}, false
}

func sessionGroupFailoverRedisKey(tokenID int, session channelAffinitySession) string {
	rule := session.RuleName
	if rule == "" {
		rule = "default"
	}
	ruleHash := affinityFingerprint(rule)
	return fmt.Sprintf("%s:%d:%s:%s", sessionGroupFailoverNamespace, tokenID, ruleHash, session.KeyFingerprint)
}

func readSessionGroupFailoverState(redisKey string) (SessionGroupFailoverState, bool) {
	if redisKey == "" || !common.RedisEnabled || common.RDB == nil {
		return SessionGroupFailoverState{}, false
	}
	raw, err := common.RedisGet(redisKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return SessionGroupFailoverState{}, false
	}
	var state SessionGroupFailoverState
	if err := common.UnmarshalJsonStr(raw, &state); err != nil {
		common.SysLog("failed to unmarshal session group failover state: " + err.Error())
		return SessionGroupFailoverState{}, false
	}
	return state, true
}

func writeSessionGroupFailoverState(redisKey string, state SessionGroupFailoverState, ttlSeconds int) error {
	if redisKey == "" || !common.RedisEnabled || common.RDB == nil {
		return nil
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	state.UpdatedAt = common.GetTimestamp()
	data, err := common.Marshal(state)
	if err != nil {
		return err
	}
	return common.RedisSet(redisKey, string(data), time.Duration(ttlSeconds)*time.Second)
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
		common.SysLog("session group failover groups parse failed: " + err.Error())
		return
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	groups, err = normalizeSessionFailoverGroups(groups, userGroup)
	if err != nil || len(groups) < 2 {
		if err != nil {
			common.SysLog("session group failover groups invalid: " + err.Error())
		}
		return
	}
	threshold := common.GetContextKeyInt(c, constant.ContextKeyTokenSessionFailoverThreshold)
	if threshold < 1 {
		threshold = 3
	}
	session, ok := resolveChannelAffinitySession(c, modelName)
	if !ok || session.KeyFingerprint == "" {
		return
	}
	tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
	if tokenID <= 0 {
		return
	}
	redisKey := sessionGroupFailoverRedisKey(tokenID, session)
	state, found := readSessionGroupFailoverState(redisKey)
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
	selectedGroup := groups[level]
	common.SetContextKey(c, constant.ContextKeyUsingGroup, selectedGroup)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, selectedGroup)
	common.SetContextKey(c, constant.ContextKeySessionGroupFailover, &SessionGroupFailoverContext{
		Enabled:        true,
		Groups:         groups,
		CurrentLevel:   level,
		SelectedGroup:  selectedGroup,
		FailureCount:   failureCount,
		Threshold:      threshold,
		KeyFingerprint: session.KeyFingerprint,
		KeyHint:        session.KeyHint,
		RuleName:       session.RuleName,
		RedisKey:       redisKey,
		TTLSeconds:     session.TTLSeconds,
	})
}

func RecordSessionGroupFailoverResult(c *gin.Context, success bool) {
	info, ok := getSessionGroupFailoverContext(c)
	if !ok || info.RedisKey == "" {
		return
	}
	state, found := readSessionGroupFailoverState(info.RedisKey)
	if !found {
		state = SessionGroupFailoverState{
			LevelIndex:   info.CurrentLevel,
			FailureCount: info.FailureCount,
			Groups:       info.Groups,
		}
	}
	if state.LevelIndex < 0 || state.LevelIndex >= len(info.Groups) {
		state.LevelIndex = info.CurrentLevel
		state.FailureCount = info.FailureCount
	}

	state, info.Switched = nextSessionGroupFailoverState(state, *info, success)
	if err := writeSessionGroupFailoverState(info.RedisKey, state, info.TTLSeconds); err != nil {
		common.SysLog("failed to write session group failover state: " + err.Error())
		return
	}
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
	adminInfo["session_group_failover"] = map[string]interface{}{
		"enabled":        info.Enabled,
		"groups":         info.Groups,
		"current_level":  info.CurrentLevel,
		"selected_group": info.SelectedGroup,
		"failure_count":  info.FailureCount,
		"threshold":      info.Threshold,
		"switched":       info.Switched,
		"key_fp":         info.KeyFingerprint,
		"key_hint":       info.KeyHint,
		"rule_name":      info.RuleName,
	}
}
