package controller

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		// Allocate a separate slice so user-specific filtering cannot modify the shared pricing cache.
		groups := make([]string, 0, len(item.EnableGroup))
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok || group == "all" {
				if ratio := ratio_setting.SubscriptionModelRatio(group, item.ModelName); ratio != nil {
					if item.SubscriptionModelRatios == nil {
						item.SubscriptionModelRatios = make(map[string]float64)
					}
					item.SubscriptionModelRatios[group] = *ratio
				}
				groups = append(groups, group)
			}
		}
		if len(groups) > 0 {
			item.EnableGroup = groups
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
		}
	}
	groupRatio := map[string]float64{}
	groupTimedRatio := map[string]*types.GroupTimedRatioSnapshot{}
	for g := range ratio_setting.GetGroupRatioCopy() {
		info := service.GetUserGroupRatioInfo(group, g)
		groupRatio[g] = info.GroupRatio
		if info.TimedRatio != nil {
			groupTimedRatio[g] = info.TimedRatio
		}
	}

	usableGroup = service.GetUserUsableGroups(group)
	// User groups classify accounts; they are not public pricing options.
	for g := range usableGroup {
		if ratio_setting.GetGroupType(g) == ratio_setting.GroupTypeUser {
			delete(usableGroup, g)
		}
	}
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	// check groupRatio contains usableGroup
	for group := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
			delete(groupTimedRatio, group)
		}
	}
	autoGroups := make([]string, 0)
	for _, g := range service.GetUserAutoGroup(group) {
		if _, ok := usableGroup[g]; ok {
			autoGroups = append(autoGroups, g)
		}
	}

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"group_timed_ratio":  groupTimedRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        autoGroups,
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
