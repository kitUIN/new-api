package ratio_setting

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type GroupRatioChangeType string

const (
	GroupRatioChangeAdded   GroupRatioChangeType = "added"
	GroupRatioChangeUpdated GroupRatioChangeType = "updated"
	GroupRatioChangeDeleted GroupRatioChangeType = "deleted"
)

type GroupRatioChange struct {
	Type     GroupRatioChangeType
	Group    string
	OldRatio float64
	NewRatio float64
}

func CompareGroupRatioChanges(previous, current map[string]float64) []GroupRatioChange {
	keys := make(map[string]struct{}, len(previous)+len(current))
	for group := range previous {
		keys[group] = struct{}{}
	}
	for group := range current {
		keys[group] = struct{}{}
	}

	groups := make([]string, 0, len(keys))
	for group := range keys {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	changes := make([]GroupRatioChange, 0)
	for _, group := range groups {
		oldRatio, oldOK := previous[group]
		newRatio, newOK := current[group]
		switch {
		case !oldOK && newOK:
			changes = append(changes, GroupRatioChange{
				Type:     GroupRatioChangeAdded,
				Group:    group,
				NewRatio: newRatio,
			})
		case oldOK && !newOK:
			changes = append(changes, GroupRatioChange{
				Type:     GroupRatioChangeDeleted,
				Group:    group,
				OldRatio: oldRatio,
			})
		case oldOK && newOK && oldRatio != newRatio:
			changes = append(changes, GroupRatioChange{
				Type:     GroupRatioChangeUpdated,
				Group:    group,
				OldRatio: oldRatio,
				NewRatio: newRatio,
			})
		}
	}
	return changes
}

func FormatGroupRatioChangeLines(changes []GroupRatioChange) []string {
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		switch change.Type {
		case GroupRatioChangeAdded:
			lines = append(lines, fmt.Sprintf("+ %s: 倍率 %g", change.Group, change.NewRatio))
		case GroupRatioChangeUpdated:
			lines = append(lines, fmt.Sprintf("* %s: %g -> %g", change.Group, change.OldRatio, change.NewRatio))
		case GroupRatioChangeDeleted:
			lines = append(lines, fmt.Sprintf("- %s: 原倍率 %g", change.Group, change.OldRatio))
		}
	}
	return lines
}

func FormatGroupRatioChangeMessage(changes []GroupRatioChange) string {
	lines := FormatGroupRatioChangeLines(changes)
	if len(lines) == 0 {
		return ""
	}
	return "分组倍率发生变化：\n" + strings.Join(lines, "\n")
}

type UpstreamGroupRatioBindingChangeType string

const (
	UpstreamGroupRatioBindingChangeAdded   UpstreamGroupRatioBindingChangeType = "added"
	UpstreamGroupRatioBindingChangeUpdated UpstreamGroupRatioBindingChangeType = "updated"
	UpstreamGroupRatioBindingChangeDeleted UpstreamGroupRatioBindingChangeType = "deleted"
)

type UpstreamGroupRatioBindingChange struct {
	Type       UpstreamGroupRatioBindingChangeType
	Group      string
	OldBinding UpstreamGroupRatioBinding
	NewBinding UpstreamGroupRatioBinding
}

func CompareUpstreamGroupRatioBindingChanges(previous, current map[string]UpstreamGroupRatioBinding) []UpstreamGroupRatioBindingChange {
	keys := make(map[string]struct{}, len(previous)+len(current))
	for group := range previous {
		keys[group] = struct{}{}
	}
	for group := range current {
		keys[group] = struct{}{}
	}

	groups := make([]string, 0, len(keys))
	for group := range keys {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	changes := make([]UpstreamGroupRatioBindingChange, 0)
	for _, group := range groups {
		oldBinding, oldOK := previous[group]
		newBinding, newOK := current[group]
		switch {
		case !oldOK && newOK:
			changes = append(changes, UpstreamGroupRatioBindingChange{
				Type:       UpstreamGroupRatioBindingChangeAdded,
				Group:      group,
				NewBinding: newBinding,
			})
		case oldOK && !newOK:
			changes = append(changes, UpstreamGroupRatioBindingChange{
				Type:       UpstreamGroupRatioBindingChangeDeleted,
				Group:      group,
				OldBinding: oldBinding,
			})
		case oldOK && newOK && !reflect.DeepEqual(oldBinding, newBinding):
			changes = append(changes, UpstreamGroupRatioBindingChange{
				Type:       UpstreamGroupRatioBindingChangeUpdated,
				Group:      group,
				OldBinding: oldBinding,
				NewBinding: newBinding,
			})
		}
	}
	return changes
}

func FormatUpstreamGroupRatioBinding(binding UpstreamGroupRatioBinding) string {
	if binding.Offset == 0 {
		return fmt.Sprintf("%s #%d / %s", binding.SourceType, binding.SourceID, binding.UpstreamGroup)
	}
	return fmt.Sprintf("%s #%d / %s / 偏移 %g", binding.SourceType, binding.SourceID, binding.UpstreamGroup, binding.Offset)
}

func FormatUpstreamGroupRatioBindingChangeLines(changes []UpstreamGroupRatioBindingChange) []string {
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		switch change.Type {
		case UpstreamGroupRatioBindingChangeAdded:
			lines = append(lines, fmt.Sprintf("+ %s: %s", change.Group, FormatUpstreamGroupRatioBinding(change.NewBinding)))
		case UpstreamGroupRatioBindingChangeUpdated:
			lines = append(lines, fmt.Sprintf("* %s: %s -> %s", change.Group, FormatUpstreamGroupRatioBinding(change.OldBinding), FormatUpstreamGroupRatioBinding(change.NewBinding)))
		case UpstreamGroupRatioBindingChangeDeleted:
			lines = append(lines, fmt.Sprintf("- %s: 原绑定 %s", change.Group, FormatUpstreamGroupRatioBinding(change.OldBinding)))
		}
	}
	return lines
}

func FormatUpstreamGroupRatioBindingChangeMessage(changes []UpstreamGroupRatioBindingChange) string {
	lines := FormatUpstreamGroupRatioBindingChangeLines(changes)
	if len(lines) == 0 {
		return ""
	}
	return "上游分组绑定发生变化：\n" + strings.Join(lines, "\n")
}

type GroupSpecialUsableChangeType string

const (
	GroupSpecialUsableChangeAdded   GroupSpecialUsableChangeType = "added"
	GroupSpecialUsableChangeUpdated GroupSpecialUsableChangeType = "updated"
	GroupSpecialUsableChangeDeleted GroupSpecialUsableChangeType = "deleted"
)

type GroupSpecialUsableChange struct {
	Type      GroupSpecialUsableChangeType
	UserGroup string
	Rule      string
	OldValue  string
	NewValue  string
}

func CompareGroupSpecialUsableChanges(previous, current map[string]map[string]string) []GroupSpecialUsableChange {
	userGroupKeys := make(map[string]struct{}, len(previous)+len(current))
	for userGroup := range previous {
		userGroupKeys[userGroup] = struct{}{}
	}
	for userGroup := range current {
		userGroupKeys[userGroup] = struct{}{}
	}

	userGroups := make([]string, 0, len(userGroupKeys))
	for userGroup := range userGroupKeys {
		userGroups = append(userGroups, userGroup)
	}
	sort.Strings(userGroups)

	changes := make([]GroupSpecialUsableChange, 0)
	for _, userGroup := range userGroups {
		oldRules := previous[userGroup]
		newRules := current[userGroup]
		ruleKeys := make(map[string]struct{}, len(oldRules)+len(newRules))
		for rule := range oldRules {
			ruleKeys[rule] = struct{}{}
		}
		for rule := range newRules {
			ruleKeys[rule] = struct{}{}
		}

		rules := make([]string, 0, len(ruleKeys))
		for rule := range ruleKeys {
			rules = append(rules, rule)
		}
		sort.Strings(rules)

		for _, rule := range rules {
			oldValue, oldOK := oldRules[rule]
			newValue, newOK := newRules[rule]
			switch {
			case !oldOK && newOK:
				changes = append(changes, GroupSpecialUsableChange{
					Type:      GroupSpecialUsableChangeAdded,
					UserGroup: userGroup,
					Rule:      rule,
					NewValue:  newValue,
				})
			case oldOK && !newOK:
				changes = append(changes, GroupSpecialUsableChange{
					Type:      GroupSpecialUsableChangeDeleted,
					UserGroup: userGroup,
					Rule:      rule,
					OldValue:  oldValue,
				})
			case oldOK && newOK && oldValue != newValue:
				changes = append(changes, GroupSpecialUsableChange{
					Type:      GroupSpecialUsableChangeUpdated,
					UserGroup: userGroup,
					Rule:      rule,
					OldValue:  oldValue,
					NewValue:  newValue,
				})
			}
		}
	}
	return changes
}

func FormatGroupSpecialUsableRule(rule string) string {
	switch {
	case strings.HasPrefix(rule, "-:"):
		return "不可见 " + strings.TrimPrefix(rule, "-:")
	case strings.HasPrefix(rule, "+:"):
		return "可见 " + strings.TrimPrefix(rule, "+:")
	default:
		return "可见 " + rule
	}
}

func FormatGroupSpecialUsableChangeLines(changes []GroupSpecialUsableChange) []string {
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		rule := FormatGroupSpecialUsableRule(change.Rule)
		switch change.Type {
		case GroupSpecialUsableChangeAdded:
			lines = append(lines, fmt.Sprintf("+ 用户分组 %s: %s (%s)", change.UserGroup, rule, change.NewValue))
		case GroupSpecialUsableChangeUpdated:
			lines = append(lines, fmt.Sprintf("* 用户分组 %s: %s 描述 %s -> %s", change.UserGroup, rule, change.OldValue, change.NewValue))
		case GroupSpecialUsableChangeDeleted:
			lines = append(lines, fmt.Sprintf("- 用户分组 %s: 移除规则 %s", change.UserGroup, rule))
		}
	}
	return lines
}

func FormatGroupSpecialUsableChangeMessage(changes []GroupSpecialUsableChange) string {
	lines := FormatGroupSpecialUsableChangeLines(changes)
	if len(lines) == 0 {
		return ""
	}
	return "特殊可用分组规则发生变化：\n" + strings.Join(lines, "\n")
}
