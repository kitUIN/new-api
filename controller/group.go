package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupType := c.Query("type")
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		if groupType == "" || ratio_setting.GetGroupType(groupName) == groupType {
			groupNames = append(groupNames, groupName)
		}
	}
	enabledChannelGroups, err := model.GetEnabledChannelGroupSet()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "",
		"data":        groupNames,
		"auto_groups": service.GetRuleAutoGroupAdminInfos(enabledChannelGroups),
	})
}

func GetUserGroups(c *gin.Context) {
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	groups, err := service.GetSortedUserUsableGroupInfos(userGroup)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    service.UserUsableGroupInfosToMap(groups),
		"groups":  groups,
	})
}
