package ratio_setting

import (
	"fmt"
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
