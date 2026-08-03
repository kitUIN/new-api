package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func ParseModelGroupCombinationGroups(raw string) ([]string, error) {
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

func encodeModelGroupCombinationGroups(groups []string) (string, error) {
	data, err := common.Marshal(groups)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeModelGroupCombinationGroups(groups []string, userGroup string) ([]string, error) {
	normalized := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			return nil, errors.New("模型组合分组不能为空")
		}
		if group == "auto" || IsRuleAutoGroup(group) {
			return nil, fmt.Errorf("模型组合只支持具体分组: %s", group)
		}
		if _, exists := seen[group]; exists {
			return nil, fmt.Errorf("模型组合分组重复: %s", group)
		}
		if !GroupInUserUsableGroups(userGroup, group) {
			return nil, fmt.Errorf("无权访问 %s 分组", group)
		}
		if !ratio_setting.ContainsGroupRatio(group) {
			return nil, fmt.Errorf("分组 %s 已被弃用", group)
		}
		seen[group] = struct{}{}
		normalized = append(normalized, group)
	}
	if len(normalized) < 2 {
		return nil, errors.New("模型组合至少需要 2 个分组")
	}
	return normalized, nil
}

func NormalizeTokenModelGroupCombination(token *model.Token, userGroup string) error {
	if token == nil {
		return errors.New("token is nil")
	}
	if !token.ModelGroupCombinationEnabled {
		token.ModelGroupCombinationGroups = ""
		return nil
	}
	if token.SessionGroupFailoverEnabled {
		return errors.New("模型组合不能同时启用 API Key 故障转移")
	}
	groups, err := ParseModelGroupCombinationGroups(token.ModelGroupCombinationGroups)
	if err != nil {
		return fmt.Errorf("模型组合分组格式错误: %w", err)
	}
	groups, err = normalizeModelGroupCombinationGroups(groups, userGroup)
	if err != nil {
		return err
	}
	encoded, err := encodeModelGroupCombinationGroups(groups)
	if err != nil {
		return err
	}
	token.ModelGroupCombinationGroups = encoded
	token.Group = groups[0]
	token.CrossGroupRetry = false
	token.AutoGroupMode = ""
	token.SessionFailoverGroups = ""
	token.SessionFailoverThreshold = 3
	return nil
}

func GetModelGroupCombinationGroupsFromContext(c *gin.Context) ([]string, error) {
	if c == nil || !common.GetContextKeyBool(c, constant.ContextKeyTokenModelGroupCombinationEnabled) {
		return nil, nil
	}
	raw := common.GetContextKeyString(c, constant.ContextKeyTokenModelGroupCombinationGroups)
	groups, err := ParseModelGroupCombinationGroups(raw)
	if err != nil {
		return nil, fmt.Errorf("模型组合分组格式错误: %w", err)
	}
	return groups, nil
}

// ResolveModelGroupCombination selects the first configured group with an enabled channel for the model.
func ResolveModelGroupCombination(c *gin.Context, modelName string) (string, bool, error) {
	if c == nil || !common.GetContextKeyBool(c, constant.ContextKeyTokenModelGroupCombinationEnabled) {
		return "", false, nil
	}
	groups, err := GetModelGroupCombinationGroupsFromContext(c)
	if err != nil {
		return "", true, err
	}
	for _, group := range groups {
		if model.HasAvailableChannelForGroupModel(group, modelName) {
			common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
			common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
			return group, true, nil
		}
	}
	return "", true, fmt.Errorf("模型组合中没有支持模型 %s 的可用分组", modelName)
}
