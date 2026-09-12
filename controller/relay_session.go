package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func getRelaySessions(c *gin.Context, userID int) {
	page := common.GetPageQuery(c)
	items, total, err := model.GetActiveRelaySessions(userID, strings.TrimSpace(c.Query("search")), page.GetStartIdx(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetItems(items)
	page.SetTotal(int(total))
	common.ApiSuccess(c, page)
}

func GetAllRelaySessions(c *gin.Context) {
	getRelaySessions(c, 0)
}

func GetSelfRelaySessions(c *gin.Context) {
	if c.GetInt("id") <= 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	getRelaySessions(c, c.GetInt("id"))
}

func GetRelaySessionGroups(c *gin.Context) {
	session, err := model.GetActiveRelaySession(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groups, err := service.GetRelaySessionGroups(session)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}

func UpdateRelaySessionGroup(c *gin.Context) {
	var request struct {
		Group *string `json:"group"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Group == nil {
		common.ApiErrorMsg(c, "分组参数不能为空")
		return
	}
	session, err := model.GetActiveRelaySession(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group := strings.TrimSpace(*request.Group)
	if err := service.SetRelaySessionGroup(session, group); err != nil {
		common.ApiError(c, err)
		return
	}
	previousGroup := session.OverrideGroup
	if previousGroup == "" {
		previousGroup = session.LastGroup
	}
	nextGroup := group
	if nextGroup == "" {
		nextGroup = "自动分组"
	}
	model.RecordLog(session.UserID, model.LogTypeManage, fmt.Sprintf("管理员 #%d 修改会话 %s 分组: %s -> %s", c.GetInt("id"), session.ID, previousGroup, nextGroup))
	common.ApiSuccess(c, nil)
}
