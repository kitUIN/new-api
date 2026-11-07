package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func serviceProbeResponse(c *gin.Context, resource string) {
	c.Header("Cache-Control", "no-store")
	data, err := service.ReadServiceProbe(c.Request.Context(), resource, c.Param("id"), c.DefaultQuery("window", "24h"))
	if errors.Is(err, service.ErrServiceProbeNotConfigured) && resource == "quotas" {
		common.ApiSuccess(c, gin.H{"configured": false, "quotas": nil})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if resource == "quotas" {
		common.ApiSuccess(c, gin.H{"configured": true, "quotas": data})
		return
	}
	common.ApiSuccess(c, data)
}

func GetServiceProbes(c *gin.Context)       { serviceProbeResponse(c, "quotas") }
func GetServiceProbeCurrent(c *gin.Context) { serviceProbeResponse(c, "current") }
func GetServiceProbeHistory(c *gin.Context) { serviceProbeResponse(c, "history") }

func GetServiceProbeSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	view, err := service.GetServiceProbeConfigView()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func SaveServiceProbeSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var input struct {
		Enabled    *bool  `json:"enabled"`
		URL        string `json:"url"`
		ReadAPIKey string `json:"read_api_key"`
	}
	if err := common.DecodeJson(c.Request.Body, &input); err != nil || input.Enabled == nil {
		common.ApiErrorMsg(c, "serviceProbe.invalidRequest")
		return
	}
	view, err := service.SaveServiceProbeSettings(*input.Enabled, input.URL, input.ReadAPIKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}
