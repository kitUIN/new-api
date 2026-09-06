package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type groupCombinationBreakerResetRequest struct {
	Group string `json:"group"`
}

func GetGroupCombinationCircuitBreakers(c *gin.Context) {
	summary, err := service.GetGroupCombinationCircuitBreakerSummary()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func ResetGroupCombinationCircuitBreaker(c *gin.Context) {
	var request groupCombinationBreakerResetRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	request.Group = strings.TrimSpace(request.Group)
	if request.Group == "" || !service.IsGroupCombinationCircuitBreakerMember(request.Group) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "目标分组不是当前组合模式成员",
		})
		return
	}
	status, err := service.ResetGroupCombinationCircuitBreaker(request.Group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}
