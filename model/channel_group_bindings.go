package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupBoundChannel struct {
	Id     int      `json:"id"`
	Name   string   `json:"name"`
	Status int      `json:"status"`
	Models []string `json:"models"`
}

// GetGroupChannelOptions includes unassigned channels and never returns keys.
func GetGroupChannelOptions() ([]GroupBoundChannel, error) {
	var channels []Channel
	if err := DB.Select("id", "name", "status", "models").Order("id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}
	options := make([]GroupBoundChannel, 0, len(channels))
	for _, channel := range channels {
		options = append(options, GroupBoundChannel{Id: channel.Id, Name: channel.Name, Status: channel.Status, Models: channel.GetModels()})
	}
	return options, nil
}

// SetGroupChannels changes only this group's membership, atomically with routing abilities.
func SetGroupChannels(group string, channelIDs []int) error {
	group = strings.TrimSpace(group)
	if group == "" || strings.Contains(group, ",") {
		return fmt.Errorf("invalid group name")
	}
	selected := make(map[int]bool, len(channelIDs))
	for _, id := range channelIDs {
		if id <= 0 {
			return fmt.Errorf("invalid channel ID")
		}
		selected[id] = true
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var channels []Channel
		query := tx.Omit("key").Order("id ASC")
		if DB.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&channels).Error; err != nil {
			return err
		}
		found := make(map[int]bool, len(channels))
		for _, channel := range channels {
			found[channel.Id] = true
		}
		for id := range selected {
			if !found[id] {
				return fmt.Errorf("channel %d does not exist", id)
			}
		}
		for _, channel := range channels {
			groups := make([]string, 0)
			hasGroup := false
			for _, existing := range channel.GetGroups() {
				if existing == group {
					hasGroup = true
				} else if existing != "" {
					groups = append(groups, existing)
				}
			}
			if hasGroup == selected[channel.Id] {
				continue
			}
			if selected[channel.Id] {
				groups = append(groups, group)
			}
			channel.Group = strings.Join(groups, ",")
			if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("group", channel.Group).Error; err != nil {
				return err
			}
			if err := channel.UpdateAbilities(tx); err != nil {
				return err
			}
		}
		return nil
	})
}

func GetChannelGroupBindings() (map[string][]GroupBoundChannel, error) {
	var channels []Channel
	err := DB.Model(&Channel{}).
		Select("id", "name", "status", "models", commonGroupCol).
		Order("id ASC").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}

	bindings := make(map[string][]GroupBoundChannel)
	for _, channel := range channels {
		seenGroups := make(map[string]struct{})
		for _, group := range channel.GetGroups() {
			group = strings.TrimSpace(group)
			if group == "" {
				continue
			}
			if _, ok := seenGroups[group]; ok {
				continue
			}
			seenGroups[group] = struct{}{}
			bindings[group] = append(bindings[group], GroupBoundChannel{
				Id:     channel.Id,
				Name:   channel.Name,
				Status: channel.Status,
				Models: channel.GetModels(),
			})
		}
	}

	return bindings, nil
}
