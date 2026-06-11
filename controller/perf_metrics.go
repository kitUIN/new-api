package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	defaultPerfMetricsHours                = 24
	defaultPerfMetricsLimit                = 200
	maxPerfMetricsHours                    = 168
	maxPerfMetricsLimit                    = 500
	defaultPerfMetricsGroupIntervalMinutes = 10
	maxPerfMetricsGroupIntervalMinutes     = 60
)

func GetPerfMetricsSummary(c *gin.Context) {
	hours := getPerfMetricsHours(c)
	limit := getPerfMetricsLimit(c)

	summary, err := model.GetPerfMetricsSummary(hours, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, summary)
}

func GetPerfGroupHealthSummary(c *gin.Context) {
	hours := getPerfMetricsHours(c)
	intervalMinutes := getPerfMetricsGroupIntervalMinutes(c)

	summary, err := model.GetPerfGroupHealthSummary(hours, intervalMinutes)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, summary)
}

func GetPerfMetrics(c *gin.Context) {
	modelName := strings.TrimSpace(c.Query("model"))
	if modelName == "" {
		common.ApiErrorMsg(c, "model is required")
		return
	}

	hours := getPerfMetricsHours(c)
	metrics, err := model.GetPerformanceMetrics(modelName, hours)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, metrics)
}

func getPerfMetricsHours(c *gin.Context) int {
	hours, err := strconv.Atoi(c.DefaultQuery("hours", strconv.Itoa(defaultPerfMetricsHours)))
	if err != nil || hours <= 0 {
		return defaultPerfMetricsHours
	}
	if hours > maxPerfMetricsHours {
		return maxPerfMetricsHours
	}
	return hours
}

func getPerfMetricsLimit(c *gin.Context) int {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultPerfMetricsLimit)))
	if err != nil || limit <= 0 {
		return defaultPerfMetricsLimit
	}
	if limit > maxPerfMetricsLimit {
		return maxPerfMetricsLimit
	}
	return limit
}

func getPerfMetricsGroupIntervalMinutes(c *gin.Context) int {
	intervalMinutes, err := strconv.Atoi(c.DefaultQuery("interval_minutes", strconv.Itoa(defaultPerfMetricsGroupIntervalMinutes)))
	if err != nil || intervalMinutes <= 0 {
		return defaultPerfMetricsGroupIntervalMinutes
	}
	if intervalMinutes > maxPerfMetricsGroupIntervalMinutes {
		return maxPerfMetricsGroupIntervalMinutes
	}
	return intervalMinutes
}
