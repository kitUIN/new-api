package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	RuleAutoGroupCodexLow = constant.RuleAutoGroupCodexLow
	RuleAutoGroupCodex    = constant.RuleAutoGroupCodex
	RuleAutoGroupCodexPro = constant.RuleAutoGroupCodexPro
	RuleAutoGroupKiro     = constant.RuleAutoGroupKiro
	RuleAutoGroupGemini   = constant.RuleAutoGroupGemini

	RuleAutoGroupModeLowRatio = constant.RuleAutoGroupModeLowRatio
	RuleAutoGroupModeBalanced = constant.RuleAutoGroupModeBalanced
)

type RuleAutoGroupRatioRange struct {
	Min          float64 `json:"min"`
	Max          float64 `json:"max"`
	MinInclusive bool    `json:"min_inclusive"`
	MaxInclusive bool    `json:"max_inclusive"`
	Display      string  `json:"display"`
}

type RuleAutoGroupInfo struct {
	Name          string                   `json:"name"`
	Label         string                   `json:"label"`
	Description   string                   `json:"description"`
	Prefix        string                   `json:"prefix"`
	AutoGroupType string                   `json:"auto_group_type"`
	RatioRange    *RuleAutoGroupRatioRange `json:"ratio_range,omitempty"`
	MatchedGroups []string                 `json:"matched_groups"`
}

func NormalizeTokenRuleAutoGroup(token *model.Token, userGroup string) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	selector := NormalizeRuleAutoGroupName(token.Group)
	if selector == "" {
		token.AutoGroupMode = ""
		return nil
	}
	token.Group = selector
	if len(GetRuleAutoGroupCandidates(userGroup, token.Group)) == 0 {
		return fmt.Errorf("无权访问 %s 分组", token.Group)
	}
	mode, err := NormalizeRuleAutoGroupMode(token.AutoGroupMode)
	if err != nil {
		return err
	}
	if token.SessionGroupFailoverEnabled {
		return fmt.Errorf("规则型自动分组不能同时启用 API Key 故障转移")
	}
	token.AutoGroupMode = mode
	token.CrossGroupRetry = false
	token.SessionFailoverGroups = ""
	token.SessionFailoverThreshold = 3
	return nil
}

type ruleAutoGroupDefinition struct {
	Name          string
	Label         string
	Description   string
	Prefix        string
	AutoGroupType string
	FixedRange    *RuleAutoGroupRatioRange
	Match         func(group string, ratio float64) bool
}

var ruleAutoGroupDefinitions = []ruleAutoGroupDefinition{
	{
		Name:          RuleAutoGroupCodexLow,
		Label:         RuleAutoGroupCodexLow,
		Description:   "自动选择 codex 开头且有效倍率在 [0, 0.1) 的分组",
		Prefix:        "codex",
		AutoGroupType: "codex_low",
		FixedRange: &RuleAutoGroupRatioRange{
			Min:          0,
			Max:          0.1,
			MinInclusive: true,
			MaxInclusive: false,
			Display:      formatRuleAutoGroupRatioRange(0, 0.1),
		},
		Match: func(group string, ratio float64) bool {
			return strings.HasPrefix(group, "codex") && ratio >= 0 && ratio < 0.1
		},
	},
	{
		Name:          RuleAutoGroupCodex,
		Label:         RuleAutoGroupCodex,
		Description:   "自动选择全部 codex 开头的分组",
		Prefix:        "codex",
		AutoGroupType: "codex",
		Match: func(group string, _ float64) bool {
			return strings.HasPrefix(group, "codex")
		},
	},
	{
		Name:          RuleAutoGroupCodexPro,
		Label:         RuleAutoGroupCodexPro,
		Description:   "自动选择 codex-pro 开头的分组",
		Prefix:        "codex-pro",
		AutoGroupType: "codex_pro",
		Match: func(group string, _ float64) bool {
			return strings.HasPrefix(group, "codex-pro")
		},
	},
	{
		Name:          RuleAutoGroupKiro,
		Label:         RuleAutoGroupKiro,
		Description:   "自动选择 cc-kiro 开头的分组",
		Prefix:        "cc-kiro",
		AutoGroupType: "kiro",
		Match: func(group string, _ float64) bool {
			return strings.HasPrefix(group, "cc-kiro")
		},
	},
	{
		Name:          RuleAutoGroupGemini,
		Label:         RuleAutoGroupGemini,
		Description:   "自动选择 gemini 开头的分组",
		Prefix:        "gemini",
		AutoGroupType: "gemini",
		Match: func(group string, _ float64) bool {
			return strings.HasPrefix(group, "gemini")
		},
	},
}

func IsRuleAutoGroup(group string) bool {
	return constant.IsRuleAutoGroup(group)
}

func NormalizeRuleAutoGroupName(group string) string {
	return constant.NormalizeRuleAutoGroupName(group)
}

func NormalizeRuleAutoGroupMode(mode string) (string, error) {
	switch strings.TrimSpace(mode) {
	case "", RuleAutoGroupModeLowRatio:
		return RuleAutoGroupModeLowRatio, nil
	case RuleAutoGroupModeBalanced:
		return RuleAutoGroupModeBalanced, nil
	default:
		return "", fmt.Errorf("不支持的自动分组模式: %s", mode)
	}
}

func getRuleAutoGroupDefinition(group string) (ruleAutoGroupDefinition, bool) {
	group = NormalizeRuleAutoGroupName(group)
	for _, definition := range ruleAutoGroupDefinitions {
		if definition.Name == group {
			return definition, true
		}
	}
	return ruleAutoGroupDefinition{}, false
}

func GetRuleAutoGroupCandidates(userGroup string, selector string) []string {
	definition, ok := getRuleAutoGroupDefinition(selector)
	if !ok {
		return nil
	}
	usableGroups := GetUserUsableGroups(userGroup)
	candidates := make([]string, 0)
	for group := range usableGroups {
		if group == "auto" || IsRuleAutoGroup(group) || !ratio_setting.ContainsGroupRatio(group) {
			continue
		}
		ratio := GetUserGroupRatio(userGroup, group)
		if definition.Match(group, ratio) {
			candidates = append(candidates, group)
		}
	}
	sortRuleAutoGroupCandidates(candidates, userGroup)
	return candidates
}

func GetRuleAutoGroupCandidatesForModel(userGroup string, selector string, modelName string) []string {
	candidates := GetRuleAutoGroupCandidates(userGroup, selector)
	if strings.TrimSpace(modelName) == "" {
		return candidates
	}
	filtered := make([]string, 0, len(candidates))
	for _, group := range candidates {
		if model.HasAvailableChannelForGroupModel(group, modelName) {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func GetRuleAutoGroupInfosForUser(userGroup string, enabledGroups map[string]bool) []RuleAutoGroupInfo {
	infos := make([]RuleAutoGroupInfo, 0, len(ruleAutoGroupDefinitions))
	for _, definition := range ruleAutoGroupDefinitions {
		candidates := GetRuleAutoGroupCandidates(userGroup, definition.Name)
		if enabledGroups != nil {
			filtered := candidates[:0]
			for _, group := range candidates {
				if enabledGroups[group] {
					filtered = append(filtered, group)
				}
			}
			candidates = filtered
		}
		if len(candidates) == 0 {
			continue
		}
		infos = append(infos, buildRuleAutoGroupInfo(definition, candidates, userGroup))
	}
	return infos
}

func GetRuleAutoGroupAdminInfos() []RuleAutoGroupInfo {
	infos := make([]RuleAutoGroupInfo, 0, len(ruleAutoGroupDefinitions))
	groupRatios := ratio_setting.GetGroupRatioCopy()
	for _, definition := range ruleAutoGroupDefinitions {
		candidates := make([]string, 0)
		for group, ratio := range groupRatios {
			if definition.Match(group, ratio) {
				candidates = append(candidates, group)
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			leftRatio := ratio_setting.GetGroupRatio(candidates[i])
			rightRatio := ratio_setting.GetGroupRatio(candidates[j])
			if leftRatio != rightRatio {
				return leftRatio < rightRatio
			}
			return candidates[i] < candidates[j]
		})
		infos = append(infos, buildRuleAutoGroupInfo(definition, candidates, ""))
	}
	return infos
}

func sortRuleAutoGroupCandidates(candidates []string, userGroup string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		leftRatio := GetUserGroupRatio(userGroup, candidates[i])
		rightRatio := GetUserGroupRatio(userGroup, candidates[j])
		if leftRatio != rightRatio {
			return leftRatio < rightRatio
		}
		return candidates[i] < candidates[j]
	})
}

func buildRuleAutoGroupInfo(definition ruleAutoGroupDefinition, candidates []string, userGroup string) RuleAutoGroupInfo {
	ratioRange := definition.FixedRange
	if ratioRange == nil && len(candidates) > 0 {
		minRatio := GetUserGroupRatio(userGroup, candidates[0])
		maxRatio := minRatio
		for _, group := range candidates[1:] {
			ratio := GetUserGroupRatio(userGroup, group)
			if ratio < minRatio {
				minRatio = ratio
			}
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
		ratioRange = &RuleAutoGroupRatioRange{
			Min:          minRatio,
			Max:          maxRatio,
			MinInclusive: true,
			MaxInclusive: true,
			Display:      formatRuleAutoGroupRatioRange(minRatio, maxRatio),
		}
	}
	if ratioRange != nil {
		clonedRange := *ratioRange
		ratioRange = &clonedRange
	}
	return RuleAutoGroupInfo{
		Name:          definition.Name,
		Label:         definition.Label,
		Description:   definition.Description,
		Prefix:        definition.Prefix,
		AutoGroupType: definition.AutoGroupType,
		RatioRange:    ratioRange,
		MatchedGroups: append([]string(nil), candidates...),
	}
}

func formatRuleAutoGroupRatio(ratio float64) string {
	if ratio == 0 {
		return "0"
	}
	return strconv.FormatFloat(ratio, 'f', -1, 64)
}

func formatRuleAutoGroupRatioRange(minRatio, maxRatio float64) string {
	return fmt.Sprintf("%sx~%sx", formatRuleAutoGroupRatio(minRatio), formatRuleAutoGroupRatio(maxRatio))
}
