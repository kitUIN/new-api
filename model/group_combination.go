package model

import (
	"sort"
	"strconv"
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

func getEnabledGroupCombinationAbilities(abilities []AbilityWithChannel) ([]AbilityWithChannel, error) {
	combinations := ratio_setting.GetGroupCombinationsCopy()
	legacyCombinations := ratio_setting.GetLegacyGroupCombinationsCopy()
	if len(combinations) == 0 && len(legacyCombinations) == 0 {
		return []AbilityWithChannel{}, nil
	}

	abilitiesByGroup := make(map[string][]AbilityWithChannel)
	for _, ability := range abilities {
		abilitiesByGroup[ability.Group] = append(abilitiesByGroup[ability.Group], ability)
	}

	combinationAbilities := make([]AbilityWithChannel, 0)
	for combinationGroup, members := range combinations {
		seen := make(map[string]struct{})
		for _, ability := range abilitiesByGroup[combinationGroup] {
			key := ability.Model + "\x00" + strconv.Itoa(ability.ChannelId)
			seen[key] = struct{}{}
		}
		for _, member := range members {
			for _, ability := range abilitiesByGroup[member.Group] {
				if !ratio_setting.GroupCombinationMemberSupportsModel(member, ability.Model) {
					continue
				}
				key := ability.Model + "\x00" + strconv.Itoa(ability.ChannelId)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				ability.Group = combinationGroup
				combinationAbilities = append(combinationAbilities, ability)
			}
		}
	}
	legacyChannelIDs := make([]int, 0)
	seenLegacyChannelIDs := make(map[int]struct{})
	for _, routes := range legacyCombinations {
		for _, channelID := range routes {
			if _, exists := seenLegacyChannelIDs[channelID]; exists {
				continue
			}
			seenLegacyChannelIDs[channelID] = struct{}{}
			legacyChannelIDs = append(legacyChannelIDs, channelID)
		}
	}
	legacyChannelsByID := make(map[int]*Channel, len(legacyChannelIDs))
	if len(legacyChannelIDs) > 0 {
		var legacyChannels []Channel
		if err := DB.Where("id IN ? AND status = ?", legacyChannelIDs, common.ChannelStatusEnabled).Find(&legacyChannels).Error; err != nil {
			return nil, err
		}
		for i := range legacyChannels {
			legacyChannelsByID[legacyChannels[i].Id] = &legacyChannels[i]
		}
	}
	for combinationGroup, routes := range legacyCombinations {
		for modelName, channelID := range routes {
			channel := legacyChannelsByID[channelID]
			if channel == nil || !channelExposesRequestedModel(channel, modelName) {
				continue
			}
			combinationAbilities = append(combinationAbilities, AbilityWithChannel{
				Ability: Ability{
					Group:     combinationGroup,
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
	return combinationAbilities, nil
}

func GetGroupCombinationEnabledModels(group string) []string {
	if ratio_setting.IsLegacyGroupCombination(group) {
		routes := ratio_setting.GetLegacyGroupCombinationsCopy()[group]
		models := make([]string, 0, len(routes))
		for modelName, channelID := range routes {
			channel, err := CacheGetChannel(channelID)
			if err == nil && channel != nil && channel.Status == common.ChannelStatusEnabled && channelExposesRequestedModel(channel, modelName) {
				models = append(models, modelName)
			}
		}
		sort.Strings(models)
		return models
	}

	members, configured := ratio_setting.GetGroupCombinationMembers(group)
	if !configured {
		return nil
	}

	models := make([]string, 0)
	seen := make(map[string]struct{})
	var concreteModels []string
	DB.Table("abilities").Where(channelSatisfyGroupCol()+" = ? and enabled = ?", group, true).Distinct("model").Pluck("model", &concreteModels)
	for _, modelName := range concreteModels {
		configuredForModel := false
		for _, member := range members {
			if ratio_setting.GroupCombinationMemberSupportsModel(member, modelName) {
				configuredForModel = true
				break
			}
		}
		if configuredForModel {
			continue
		}
		seen[modelName] = struct{}{}
		models = append(models, modelName)
	}
	for _, member := range members {
		for _, modelName := range member.Models {
			if !hasAvailableChannelForConcreteGroupModel(member.Group, modelName) {
				continue
			}
			if _, exists := seen[modelName]; exists {
				continue
			}
			seen[modelName] = struct{}{}
			models = append(models, modelName)
		}
	}
	sort.Strings(models)
	return models
}

func IsGroupCombinationModelAvailable(group, modelName string) bool {
	if channelID, configured, routed := ratio_setting.GetGroupCombinationChannelID(group, modelName); configured {
		if !routed {
			return false
		}
		channel, err := CacheGetChannel(channelID)
		return err == nil && channel != nil && channel.Status == common.ChannelStatusEnabled && channelExposesRequestedModel(channel, modelName)
	}

	members, configured := ratio_setting.GetGroupCombinationMembers(group)
	if !configured {
		return false
	}
	configuredForModel := false
	for _, member := range members {
		if !ratio_setting.GroupCombinationMemberSupportsModel(member, modelName) {
			continue
		}
		configuredForModel = true
		if hasAvailableChannelForConcreteGroupModel(member.Group, modelName) {
			return true
		}
	}
	return !configuredForModel && hasAvailableChannelForConcreteGroupModel(group, modelName)
}

func IsGroupCombinationChannelAvailable(group, modelName string, channelID int) bool {
	if routedChannelID, configured, routed := ratio_setting.GetGroupCombinationChannelID(group, modelName); configured {
		if !routed || routedChannelID != channelID {
			return false
		}
		channel, err := CacheGetChannel(channelID)
		return err == nil && channel != nil && channel.Status == common.ChannelStatusEnabled && channelExposesRequestedModel(channel, modelName)
	}

	members, configured := ratio_setting.GetGroupCombinationMembers(group)
	if !configured {
		return false
	}
	configuredForModel := false
	for _, member := range members {
		if !ratio_setting.GroupCombinationMemberSupportsModel(member, modelName) {
			continue
		}
		configuredForModel = true
		if isChannelEnabledForConcreteGroupModel(member.Group, modelName, channelID) {
			return true
		}
	}
	return !configuredForModel && isChannelEnabledForConcreteGroupModel(group, modelName, channelID)
}
