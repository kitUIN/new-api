package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetChannelGroupBindings(c *gin.Context) {
	bindings, err := model.GetChannelGroupBindings()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    bindings,
	})
}

func GetGroupChannelOptions(c *gin.Context) {
	channels, err := model.GetGroupChannelOptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": channels})
}

func SetGroupChannels(c *gin.Context) {
	var request struct {
		Group      string `json:"group"`
		ChannelIDs *[]int `json:"channel_ids"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.ChannelIDs == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "group and channel_ids are required"})
		return
	}
	request.Group = strings.TrimSpace(request.Group)
	_, exists := ratio_setting.GetGroupRatioCopy()[request.Group]
	if !exists || ratio_setting.GetGroupType(request.Group) != ratio_setting.GroupTypeBilling || ratio_setting.IsGroupCombination(request.Group) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Save a standard billing group before assigning channels"})
		return
	}
	if err := model.SetGroupChannels(request.Group, *request.ChannelIDs); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
