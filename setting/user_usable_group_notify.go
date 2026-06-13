package setting

import (
	"fmt"
	"sort"
	"strings"
)

type UserUsableGroupChangeType string

const (
	UserUsableGroupChangeAdded       UserUsableGroupChangeType = "added"
	UserUsableGroupChangeUpdated     UserUsableGroupChangeType = "updated"
	UserUsableGroupChangeDeleted     UserUsableGroupChangeType = "deleted"
	UserUsableGroupChangeDescription UserUsableGroupChangeType = "description"
)

type UserUsableGroupChange struct {
	Type        UserUsableGroupChangeType
	Group       string
	OldDesc     string
	NewDesc     string
	OldDisabled bool
	NewDisabled bool
}

type userUsableGroupNotifyState struct {
	desc     string
	visible  bool
	disabled bool
}

func normalizeUserUsableGroupNotifyState(groups map[string]string) map[string]userUsableGroupNotifyState {
	states := make(map[string]userUsableGroupNotifyState, len(groups))
	for group, desc := range groups {
		if isDisabledGroupDescriptionKey(group) {
			name := strings.TrimPrefix(group, disabledGroupDescriptionPrefix)
			if strings.TrimSpace(name) == "" {
				continue
			}
			state := states[name]
			if !state.visible {
				state.desc = desc
			}
			state.disabled = true
			states[name] = state
			continue
		}
		state := states[group]
		state.desc = desc
		state.visible = true
		states[group] = state
	}
	return states
}

func CompareUserUsableGroupChanges(previous, current map[string]string) []UserUsableGroupChange {
	oldStates := normalizeUserUsableGroupNotifyState(previous)
	newStates := normalizeUserUsableGroupNotifyState(current)

	keys := make(map[string]struct{}, len(oldStates)+len(newStates))
	for group := range oldStates {
		keys[group] = struct{}{}
	}
	for group := range newStates {
		keys[group] = struct{}{}
	}

	groups := make([]string, 0, len(keys))
	for group := range keys {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	changes := make([]UserUsableGroupChange, 0)
	for _, group := range groups {
		oldState, oldOK := oldStates[group]
		newState, newOK := newStates[group]
		switch {
		case !oldOK && newOK:
			changes = append(changes, UserUsableGroupChange{
				Type:        UserUsableGroupChangeAdded,
				Group:       group,
				NewDesc:     newState.desc,
				NewDisabled: newState.disabled && !newState.visible,
			})
		case oldOK && !newOK:
			changes = append(changes, UserUsableGroupChange{
				Type:        UserUsableGroupChangeDeleted,
				Group:       group,
				OldDesc:     oldState.desc,
				OldDisabled: oldState.disabled && !oldState.visible,
			})
		case oldOK && newOK && oldState.visible != newState.visible:
			changes = append(changes, UserUsableGroupChange{
				Type:        UserUsableGroupChangeUpdated,
				Group:       group,
				OldDesc:     oldState.desc,
				NewDesc:     newState.desc,
				OldDisabled: oldState.disabled && !oldState.visible,
				NewDisabled: newState.disabled && !newState.visible,
			})
		case oldOK && newOK && oldState.desc != newState.desc:
			changes = append(changes, UserUsableGroupChange{
				Type:        UserUsableGroupChangeDescription,
				Group:       group,
				OldDesc:     oldState.desc,
				NewDesc:     newState.desc,
				OldDisabled: oldState.disabled && !oldState.visible,
				NewDisabled: newState.disabled && !newState.visible,
			})
		}
	}
	return changes
}

func FormatUserUsableGroupChangeLines(changes []UserUsableGroupChange) []string {
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		switch change.Type {
		case UserUsableGroupChangeAdded:
			if change.NewDisabled {
				lines = append(lines, fmt.Sprintf("+ %s: 用户不可见，描述 %s", change.Group, change.NewDesc))
			} else {
				lines = append(lines, fmt.Sprintf("+ %s: 用户可见，描述 %s", change.Group, change.NewDesc))
			}
		case UserUsableGroupChangeUpdated:
			if change.NewDisabled {
				lines = append(lines, fmt.Sprintf("* %s: 用户可见 -> 用户不可见", change.Group))
			} else {
				lines = append(lines, fmt.Sprintf("* %s: 用户不可见 -> 用户可见", change.Group))
			}
		case UserUsableGroupChangeDeleted:
			if change.OldDisabled {
				lines = append(lines, fmt.Sprintf("- %s: 移除不可见描述", change.Group))
			} else {
				lines = append(lines, fmt.Sprintf("- %s: 用户不再可见", change.Group))
			}
		case UserUsableGroupChangeDescription:
			lines = append(lines, fmt.Sprintf("* %s: 描述 %s -> %s", change.Group, change.OldDesc, change.NewDesc))
		}
	}
	return lines
}

func FormatUserUsableGroupChangeMessage(changes []UserUsableGroupChange) string {
	lines := FormatUserUsableGroupChangeLines(changes)
	if len(lines) == 0 {
		return ""
	}
	return "用户可见分组发生变化：\n" + strings.Join(lines, "\n")
}
