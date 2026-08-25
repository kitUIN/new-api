package model

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func channelExposesRequestedModel(channel *Channel, modelName string) bool {
	if channel == nil || strings.TrimSpace(modelName) == "" {
		return false
	}
	modelName = strings.TrimSpace(modelName)
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	for _, exposedModel := range channel.GetModels() {
		exposedModel = strings.TrimSpace(exposedModel)
		if exposedModel == modelName || (normalizedModel != modelName && exposedModel == normalizedModel) {
			return true
		}
	}
	return false
}

func ChannelExposesRequestedModel(channel *Channel, modelName string) bool {
	return channelExposesRequestedModel(channel, modelName)
}

func getEnabledGroupCombinationAbilities() ([]AbilityWithChannel, error) {
	combinations := ratio_setting.GetGroupCombinationsCopy()
	if len(combinations) == 0 {
		return []AbilityWithChannel{}, nil
	}

	channelIDs := make([]int, 0)
	seenChannelIDs := make(map[int]struct{})
	for _, routes := range combinations {
		for _, channelID := range routes {
			if _, exists := seenChannelIDs[channelID]; exists {
				continue
			}
			seenChannelIDs[channelID] = struct{}{}
			channelIDs = append(channelIDs, channelID)
		}
	}
	if len(channelIDs) == 0 {
		return []AbilityWithChannel{}, nil
	}

	var channels []Channel
	if err := DB.Where("id IN ? AND status = ?", channelIDs, common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	channelByID := make(map[int]*Channel, len(channels))
	for i := range channels {
		channelByID[channels[i].Id] = &channels[i]
	}

	abilities := make([]AbilityWithChannel, 0)
	for group, routes := range combinations {
		for modelName, channelID := range routes {
			channel := channelByID[channelID]
			if !channelExposesRequestedModel(channel, modelName) {
				continue
			}
			abilities = append(abilities, AbilityWithChannel{
				Ability: Ability{
					Group:     group,
					Model:     modelName,
					ChannelId: channelID,
					Enabled:   true,
					Priority:  channel.Priority,
					Weight:    uint(channel.GetWeight()),
					Tag:       channel.Tag,
				},
				ChannelType: channel.Type,
			})
		}
	}
	return abilities, nil
}

func GetGroupCombinationEnabledModels(group string) []string {
	if !ratio_setting.IsGroupCombination(group) {
		return nil
	}
	abilities, err := getEnabledGroupCombinationAbilities()
	if err != nil {
		common.SysLog("failed to load combination group models: " + err.Error())
		return []string{}
	}
	models := make([]string, 0)
	seen := make(map[string]struct{})
	for _, ability := range abilities {
		if ability.Group != group {
			continue
		}
		if _, exists := seen[ability.Model]; exists {
			continue
		}
		seen[ability.Model] = struct{}{}
		models = append(models, ability.Model)
	}
	sort.Strings(models)
	return models
}

func IsGroupCombinationModelAvailable(group, modelName string) bool {
	channelID, configured, routed := ratio_setting.GetGroupCombinationChannelID(group, modelName)
	if !configured || !routed {
		return false
	}
	channel, err := CacheGetChannel(channelID)
	return err == nil && channel != nil && channel.Status == common.ChannelStatusEnabled && channelExposesRequestedModel(channel, modelName)
}

func IsGroupCombinationChannelAvailable(group, modelName string, channelID int) bool {
	routedChannelID, configured, routed := ratio_setting.GetGroupCombinationChannelID(group, modelName)
	if !configured || !routed || routedChannelID != channelID {
		return false
	}
	channel, err := CacheGetChannel(channelID)
	return err == nil && channel != nil && channel.Status == common.ChannelStatusEnabled && channelExposesRequestedModel(channel, modelName)
}
