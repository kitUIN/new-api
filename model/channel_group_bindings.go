package model

import "strings"

type GroupBoundChannel struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

func GetChannelGroupBindings() (map[string][]GroupBoundChannel, error) {
	var channels []Channel
	err := DB.Model(&Channel{}).
		Select("id", "name", "status", commonGroupCol).
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
			})
		}
	}

	return bindings, nil
}
