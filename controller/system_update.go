package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type systemUpdateRequest struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
}

func configuredUpdateRepository() string {
	common.OptionMapRWMutex.RLock()
	repository := common.OptionMap["GitHubUpdateRepository"]
	common.OptionMapRWMutex.RUnlock()
	return repository
}

func CheckSystemUpdate(c *gin.Context) {
	repository := strings.TrimSpace(c.Query("repository"))
	if repository == "" {
		repository = configuredUpdateRepository()
	}

	info, err := service.CheckSystemUpdate(c.Request.Context(), repository)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, info)
}

func ApplySystemUpdate(c *gin.Context) {
	var request systemUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(request.Repository) == "" {
		request.Repository = configuredUpdateRepository()
	}

	info, err := service.ApplySystemUpdate(c.Request.Context(), request.Repository, strings.TrimSpace(request.Version))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, info)
}
