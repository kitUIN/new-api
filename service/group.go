package service

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type UserUsableGroupInfo struct {
	Name          string                   `json:"name"`
	Label         string                   `json:"label"`
	Ratio         interface{}              `json:"ratio"`
	Desc          string                   `json:"desc"`
	IsAutoGroup   bool                     `json:"is_auto_group"`
	AutoGroupType string                   `json:"auto_group_type,omitempty"`
	RatioRange    *RuleAutoGroupRatioRange `json:"ratio_range,omitempty"`
}

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	return groupsCopy
}

func GetSortedUserUsableGroupInfos(userGroup string) ([]UserUsableGroupInfo, error) {
	userUsableGroups := GetUserUsableGroups(userGroup)
	groupRatios := ratio_setting.GetGroupRatioCopy()
	enabledChannelGroups, err := model.GetEnabledChannelGroupSet()
	if err != nil {
		return nil, err
	}

	groups := make([]UserUsableGroupInfo, 0, len(groupRatios)+1)
	for groupName := range groupRatios {
		desc, ok := userUsableGroups[groupName]
		if !ok || !enabledChannelGroups[groupName] {
			continue
		}
		groups = append(groups, UserUsableGroupInfo{
			Name:  groupName,
			Label: groupName,
			Ratio: GetUserGroupRatio(userGroup, groupName),
			Desc:  desc,
		})
	}

	sort.SliceStable(groups, func(i, j int) bool {
		leftRatio, leftOk := groups[i].Ratio.(float64)
		rightRatio, rightOk := groups[j].Ratio.(float64)
		if leftOk && rightOk && leftRatio != rightRatio {
			return leftRatio < rightRatio
		}
		return groups[i].Name < groups[j].Name
	})

	if _, ok := userUsableGroups["auto"]; ok && len(groups) > 0 {
		groups = append(groups, UserUsableGroupInfo{
			Name:        "auto",
			Label:       "auto",
			Ratio:       "自动",
			Desc:        setting.GetUsableGroupDescription("auto"),
			IsAutoGroup: true,
		})
	}

	for _, info := range GetRuleAutoGroupInfosForUser(userGroup, enabledChannelGroups) {
		ratioDisplay := ""
		if info.RatioRange != nil {
			ratioDisplay = info.RatioRange.Display
		}
		groups = append(groups, UserUsableGroupInfo{
			Name:          info.Name,
			Label:         info.Label,
			Ratio:         ratioDisplay,
			Desc:          info.Description,
			IsAutoGroup:   true,
			AutoGroupType: info.AutoGroupType,
			RatioRange:    info.RatioRange,
		})
	}

	return groups, nil
}

func UserUsableGroupInfosToMap(groups []UserUsableGroupInfo) map[string]map[string]interface{} {
	usableGroups := make(map[string]map[string]interface{}, len(groups))
	for _, group := range groups {
		usableGroups[group.Name] = map[string]interface{}{
			"label":           group.Label,
			"ratio":           group.Ratio,
			"desc":            group.Desc,
			"is_auto_group":   group.IsAutoGroup,
			"auto_group_type": group.AutoGroupType,
			"ratio_range":     group.RatioRange,
		}
	}
	return usableGroups
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	if IsRuleAutoGroup(groupName) {
		return len(GetRuleAutoGroupCandidates(userGroup, groupName)) > 0
	}
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if ratio_setting.IsGroupCombination(group) {
			continue
		}
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	if setting.GetAutoGroupOrderType() == setting.AutoGroupOrderTypeRatioAsc {
		sort.SliceStable(autoGroups, func(i, j int) bool {
			leftRatio := GetUserGroupRatio(userGroup, autoGroups[i])
			rightRatio := GetUserGroupRatio(userGroup, autoGroups[j])
			if leftRatio != rightRatio {
				return leftRatio < rightRatio
			}
			return autoGroups[i] < autoGroups[j]
		})
	}
	return autoGroups
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}
