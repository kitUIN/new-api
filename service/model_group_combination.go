package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func ParseModelGroupCombinationMembers(raw string) ([]ratio_setting.GroupCombinationMember, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, nil
	}

	var members []ratio_setting.GroupCombinationMember
	if err := common.UnmarshalJsonStr(raw, &members); err == nil {
		return members, false, nil
	}

	var groups []string
	if err := common.UnmarshalJsonStr(raw, &groups); err != nil {
		return nil, false, err
	}
	members = make([]ratio_setting.GroupCombinationMember, 0, len(groups))
	for _, group := range groups {
		members = append(members, ratio_setting.GroupCombinationMember{
			Group: group,
		})
	}
	return members, true, nil
}

func ParseModelGroupCombinationGroups(raw string) ([]string, error) {
	members, _, err := ParseModelGroupCombinationMembers(raw)
	if err != nil {
		return nil, err
	}
	groups := make([]string, 0, len(members))
	for _, member := range members {
		groups = append(groups, member.Group)
	}
	return groups, nil
}

func encodeModelGroupCombinationMembers(members []ratio_setting.GroupCombinationMember) (string, error) {
	data, err := common.Marshal(members)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func encodeModelGroupCombinationGroups(members []ratio_setting.GroupCombinationMember) (string, error) {
	groups := make([]string, 0, len(members))
	for _, member := range members {
		groups = append(groups, member.Group)
	}
	data, err := common.Marshal(groups)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeModelGroupCombinationMembers(members []ratio_setting.GroupCombinationMember, userGroup string, legacy bool) ([]ratio_setting.GroupCombinationMember, error) {
	normalized := make([]ratio_setting.GroupCombinationMember, 0, len(members))
	seenGroups := make(map[string]struct{}, len(members))
	for _, member := range members {
		group := strings.TrimSpace(member.Group)
		if group == "" {
			return nil, errors.New("模型组合分组不能为空")
		}
		if group == "auto" || IsRuleAutoGroup(group) {
			return nil, fmt.Errorf("模型组合只支持具体分组: %s", group)
		}
		if !legacy && ratio_setting.IsGroupCombination(group) {
			return nil, fmt.Errorf("模型组合不支持嵌套组合分组: %s", group)
		}
		if _, exists := seenGroups[group]; exists {
			return nil, fmt.Errorf("模型组合分组重复: %s", group)
		}
		if !GroupInUserUsableGroups(userGroup, group) {
			return nil, fmt.Errorf("无权访问 %s 分组", group)
		}
		if !ratio_setting.ContainsGroupRatio(group) {
			return nil, fmt.Errorf("分组 %s 已被弃用", group)
		}

		memberModels := member.Models
		if legacy {
			memberModels = model.GetGroupEnabledModels(group)
			sort.Strings(memberModels)
		}
		models := make([]string, 0, len(memberModels))
		seenModels := make(map[string]struct{}, len(memberModels))
		for _, modelName := range memberModels {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				return nil, fmt.Errorf("模型组合分组 %s 的模型不能为空", group)
			}
			if _, exists := seenModels[modelName]; exists {
				return nil, fmt.Errorf("模型组合分组 %s 的模型重复: %s", group, modelName)
			}
			seenModels[modelName] = struct{}{}
			models = append(models, modelName)
		}
		if len(models) == 0 && !legacy {
			return nil, fmt.Errorf("模型组合分组 %s 至少需要选择 1 个模型", group)
		}

		seenGroups[group] = struct{}{}
		normalized = append(normalized, ratio_setting.GroupCombinationMember{
			Group:  group,
			Models: models,
		})
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
	members, legacy, err := ParseModelGroupCombinationMembers(token.ModelGroupCombinationGroups)
	if err != nil {
		return fmt.Errorf("模型组合分组格式错误: %w", err)
	}
	members, err = normalizeModelGroupCombinationMembers(members, userGroup, legacy)
	if err != nil {
		return err
	}
	var encoded string
	if legacy {
		encoded, err = encodeModelGroupCombinationGroups(members)
	} else {
		encoded, err = encodeModelGroupCombinationMembers(members)
	}
	if err != nil {
		return err
	}
	token.ModelGroupCombinationGroups = encoded
	token.Group = members[0].Group
	token.CrossGroupRetry = false
	token.AutoGroupMode = ""
	token.SessionFailoverGroups = ""
	token.SessionFailoverThreshold = 3
	return nil
}

func GetModelGroupCombinationMembersFromContext(c *gin.Context) ([]ratio_setting.GroupCombinationMember, error) {
	if c == nil || !common.GetContextKeyBool(c, constant.ContextKeyTokenModelGroupCombinationEnabled) {
		return nil, nil
	}
	raw := common.GetContextKeyString(c, constant.ContextKeyTokenModelGroupCombinationGroups)
	members, legacy, err := ParseModelGroupCombinationMembers(raw)
	if err != nil {
		return nil, fmt.Errorf("模型组合分组格式错误: %w", err)
	}
	if legacy {
		for i := range members {
			members[i].Models = model.GetGroupEnabledModels(strings.TrimSpace(members[i].Group))
			sort.Strings(members[i].Models)
		}
	}
	return members, nil
}

func GetModelGroupCombinationGroupsFromContext(c *gin.Context) ([]string, error) {
	members, err := GetModelGroupCombinationMembersFromContext(c)
	if err != nil {
		return nil, err
	}
	groups := make([]string, 0, len(members))
	for _, member := range members {
		groups = append(groups, member.Group)
	}
	return groups, nil
}

// ResolveModelGroupCombination selects the first configured member that explicitly
// includes the requested model and currently has an enabled channel for it.
func ResolveModelGroupCombination(c *gin.Context, modelName string) (string, bool, error) {
	if c == nil || !common.GetContextKeyBool(c, constant.ContextKeyTokenModelGroupCombinationEnabled) {
		return "", false, nil
	}
	members, err := GetModelGroupCombinationMembersFromContext(c)
	if err != nil {
		return "", true, err
	}
	for _, member := range members {
		if !ratio_setting.GroupCombinationMemberSupportsModel(member, modelName) {
			continue
		}
		if model.HasAvailableChannelForGroupModel(member.Group, modelName) {
			common.SetContextKey(c, constant.ContextKeyUsingGroup, member.Group)
			common.SetContextKey(c, constant.ContextKeyTokenGroup, member.Group)
			return member.Group, true, nil
		}
	}
	return "", true, fmt.Errorf("模型组合中没有支持模型 %s 的可用分组", modelName)
}
