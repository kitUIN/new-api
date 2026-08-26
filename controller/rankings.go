package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type updateRankingPrivacyRequest struct {
	Public bool `json:"public"`
}

func GetRankings(c *gin.Context) {
	period := c.DefaultQuery("period", "week")
	startTime, endTime, err := rankingCustomTimeRange(c, period)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result, err := service.GetRankingsSnapshot(
		period,
		c.GetInt("id"),
		c.DefaultQuery("user_metric", string(service.RankingUserMetricTokens)),
		startTime,
		endTime,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func rankingCustomTimeRange(c *gin.Context, period string) (int64, int64, error) {
	if period != "custom" {
		return 0, 0, nil
	}

	startTime, err := strconv.ParseInt(c.Query("start_time"), 10, 64)
	if err != nil || startTime <= 0 {
		return 0, 0, fmt.Errorf("invalid ranking start_time")
	}
	endTime, err := strconv.ParseInt(c.Query("end_time"), 10, 64)
	if err != nil || endTime <= 0 {
		return 0, 0, fmt.Errorf("invalid ranking end_time")
	}
	return startTime, endTime, nil
}

func UpdateRankingPrivacy(c *gin.Context) {
	var req updateRankingPrivacyRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	user, err := model.GetUserById(c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	settings := user.GetSetting()
	settings.RankingPublic = req.Public
	user.SetSetting(settings)
	if err := user.Update(false); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUpdateFailed)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"public": settings.RankingPublic,
		},
	})
}
