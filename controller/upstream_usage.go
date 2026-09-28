package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetUpstreamUsage(c *gin.Context) {
	data, err := service.ListUpstreamUsage()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// Match the existing group-monitoring read APIs, without exposing upstream
// addresses, credentials or accounts that have no group binding.
func GetGroupUpstreamUsage(c *gin.Context) {
	providers, err := service.ListUpstreamUsage()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groups := map[string][]service.UpstreamUsageAccountView{}
	for _, provider := range providers {
		for _, account := range provider.Accounts {
			bindings := account.Groups
			account.Groups = []string{}
			for _, group := range bindings {
				groups[group] = append(groups[group], account)
			}
		}
	}
	common.ApiSuccess(c, groups)
}

func SaveUpstreamUsage(c *gin.Context) {
	id := 0
	if c.Param("id") != "" {
		value, err := strconv.Atoi(c.Param("id"))
		if err != nil || value <= 0 {
			common.ApiErrorMsg(c, "invalid upstream id")
			return
		}
		id = value
	}
	var input model.UpstreamUsageProviderInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		common.ApiErrorMsg(c, "invalid request")
		return
	}
	if err := service.SaveUpstreamUsage(id, input); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func DeleteUpstreamUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid upstream id")
		return
	}
	if err := service.DeleteUpstreamUsage(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RefreshUpstreamUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid upstream id")
		return
	}
	if err := service.PollUpstreamUsage(c.Request.Context(), id, false); err != nil {
		common.ApiError(c, err)
		return
	}
	GetUpstreamUsage(c)
}
