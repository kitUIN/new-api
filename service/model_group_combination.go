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

func modelGroupCombinationCandidates(members []ratio_setting.GroupCombinationMember, modelName string) []ratio_setting.GroupCombinationMember {
	candidates := make([]ratio_setting.GroupCombinationMember, 0, len(members))
	for _, member := range members {
		if ratio_setting.GroupCombinationMemberSupportsModel(member, modelName) {
			candidates = append(candidates, member)
		}
	}
	return candidates
}

func modelGroupCombinationRuntimeIdentity(tokenID int, members []ratio_setting.GroupCombinationMember) (string, string, string, error) {
	data, err := common.Marshal(members)
	if err != nil {
		return "", "", "", err
	}
	signature := common.Sha1(data)
	scope := fmt.Sprintf("token:%d", tokenID)
	root := strings.Join([]string{"model-combination", scope, signature}, ":")
	return root, scope, signature, nil
}

func getTokenModelGroupCombinationMembers(token *model.Token) ([]ratio_setting.GroupCombinationMember, error) {
	if token == nil || !token.ModelGroupCombinationEnabled {
		return nil, errors.New("API Key 未启用模型组合")
	}
	members, legacy, err := ParseModelGroupCombinationMembers(token.ModelGroupCombinationGroups)
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

func GetModelGroupCombinationCircuitBreakerSummary(token *model.Token) (GroupCombinationBreakerSummary, error) {
	summary := GroupCombinationBreakerSummary{
		FailureThreshold: GroupCombinationBreakerFailureThreshold,
		CooldownSeconds:  GroupCombinationBreakerCooldownSeconds,
		Groups:           make([]GroupCombinationBreakerStatus, 0),
	}
	members, err := getTokenModelGroupCombinationMembers(token)
	if err != nil {
		return summary, err
	}
	_, scope, signature, err := modelGroupCombinationRuntimeIdentity(token.Id, members)
	if err != nil {
		return summary, err
	}
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		group := strings.TrimSpace(member.Group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		status, statusErr := scopedGroupCombinationBreakerStatus(scope, group, signature)
		if statusErr != nil {
			return summary, statusErr
		}
		summary.Groups = append(summary.Groups, status)
	}
	return summary, nil
}

func ResetModelGroupCombinationCircuitBreaker(token *model.Token, group string) (GroupCombinationBreakerStatus, error) {
	members, err := getTokenModelGroupCombinationMembers(token)
	if err != nil {
		return GroupCombinationBreakerStatus{}, err
	}
	group = strings.TrimSpace(group)
	found := false
	for _, member := range members {
		if strings.TrimSpace(member.Group) == group {
			found = true
			break
		}
	}
	if !found {
		return GroupCombinationBreakerStatus{}, fmt.Errorf("分组 %s 不是当前模型组合成员", group)
	}
	_, scope, signature, err := modelGroupCombinationRuntimeIdentity(token.Id, members)
	if err != nil {
		return GroupCombinationBreakerStatus{}, err
	}
	status, err := resetScopedGroupCombinationCircuitBreaker(scope, group, signature)
	if err == nil {
		common.SysLog(fmt.Sprintf("API key model combination circuit breaker manually reset: token_id=%d group=%s", token.Id, group))
	}
	return status, err
}

func prepareModelGroupCombinationRuntime(c *gin.Context, modelName string) (*groupCombinationRuntime, bool, error) {
	if c == nil || !common.GetContextKeyBool(c, constant.ContextKeyTokenModelGroupCombinationEnabled) {
		return nil, false, nil
	}
	members, err := GetModelGroupCombinationMembersFromContext(c)
	if err != nil {
		return nil, true, err
	}
	candidates := modelGroupCombinationCandidates(members, modelName)
	if len(candidates) == 0 {
		return nil, true, fmt.Errorf("模型组合中没有配置模型 %s", modelName)
	}
	root, scope, signature, err := modelGroupCombinationRuntimeIdentity(
		common.GetContextKeyInt(c, constant.ContextKeyTokenId),
		members,
	)
	if err != nil {
		return nil, true, err
	}
	runtime := getGroupCombinationRuntimeForCandidates(
		c,
		root,
		modelName,
		candidates,
		groupCombinationSourceToken,
		scope,
		signature,
	)
	if len(runtime.Members) == 0 {
		return nil, true, fmt.Errorf("模型组合中支持模型 %s 的分组当前均已自动跳过", modelName)
	}
	return runtime, true, nil
}

func PrepareModelGroupCombination(c *gin.Context, modelName string) (bool, error) {
	_, enabled, err := prepareModelGroupCombinationRuntime(c, modelName)
	return enabled, err
}

func ResolveModelGroupCombinationChannel(c *gin.Context, modelName string, excludedChannelIDs []int) (*model.Channel, string, bool, error) {
	if c == nil {
		return nil, "", false, nil
	}
	value, ok := c.Get(ginKeyGroupCombinationRuntime)
	if !ok {
		return nil, "", false, nil
	}
	runtime, ok := value.(*groupCombinationRuntime)
	if !ok || runtime == nil || runtime.Source != groupCombinationSourceToken || runtime.ModelName != modelName {
		return nil, "", false, nil
	}
	channel, selectedGroup, err := resolveGroupCombinationRuntimeChannel(runtime, "模型组合", modelName, excludedChannelIDs)
	if channel != nil {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, selectedGroup)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, selectedGroup)
	}
	return channel, selectedGroup, true, err
}

func ResolveModelGroupCombinationChannelGroup(c *gin.Context, modelName string, channelID int) (string, bool, error) {
	if c == nil || !common.GetContextKeyBool(c, constant.ContextKeyTokenModelGroupCombinationEnabled) {
		return "", false, nil
	}
	members, err := GetModelGroupCombinationMembersFromContext(c)
	if err != nil {
		return "", true, err
	}
	for _, member := range modelGroupCombinationCandidates(members, modelName) {
		if model.IsChannelEnabledForConcreteGroupModel(member.Group, modelName, channelID) {
			common.SetContextKey(c, constant.ContextKeyUsingGroup, member.Group)
			common.SetContextKey(c, constant.ContextKeyTokenGroup, member.Group)
			return member.Group, true, nil
		}
	}
	return "", true, fmt.Errorf("模型组合的成员分组不包含渠道 #%d", channelID)
}

// ResolveModelGroupCombination selects the current session member, or the next
// configured member, that explicitly includes the requested model and has an
// enabled channel for it.
func ResolveModelGroupCombination(c *gin.Context, modelName string) (string, bool, error) {
	if c == nil || !common.GetContextKeyBool(c, constant.ContextKeyTokenModelGroupCombinationEnabled) {
		return "", false, nil
	}
	members, err := GetModelGroupCombinationMembersFromContext(c)
	if err != nil {
		return "", true, err
	}
	candidates := modelGroupCombinationCandidates(members, modelName)
	root, scope, signature, err := modelGroupCombinationRuntimeIdentity(
		common.GetContextKeyInt(c, constant.ContextKeyTokenId),
		members,
	)
	if err != nil {
		return "", true, err
	}
	availableCandidates := make([]ratio_setting.GroupCombinationMember, 0, len(candidates))
	for _, member := range candidates {
		if isScopedGroupCombinationMemberSkipped(scope, member.Group, signature) {
			continue
		}
		availableCandidates = append(availableCandidates, member)
	}
	_, _, currentIndex := groupCombinationSessionState(c, root, modelName, availableCandidates)
	for i := currentIndex; i < len(availableCandidates); i++ {
		member := availableCandidates[i]
		if model.HasAvailableChannelForGroupModel(member.Group, modelName) {
			common.SetContextKey(c, constant.ContextKeyUsingGroup, member.Group)
			common.SetContextKey(c, constant.ContextKeyTokenGroup, member.Group)
			return member.Group, true, nil
		}
	}
	if len(candidates) == 0 {
		return "", true, fmt.Errorf("模型组合中没有配置模型 %s", modelName)
	}
	if len(availableCandidates) == 0 {
		return "", true, fmt.Errorf("模型组合中支持模型 %s 的分组当前均已自动跳过", modelName)
	}
	return "", true, fmt.Errorf("模型组合中没有支持模型 %s 的可用分组", modelName)
}
