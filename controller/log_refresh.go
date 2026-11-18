package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func RefreshAllLogs(c *gin.Context) { refreshLogs(c, 0) }

func RefreshUserLogs(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	refreshLogs(c, userID)
}

func refreshLogs(c *gin.Context, userID int) {
	parts := strings.Split(c.Query("ids"), ",")
	if len(parts) > model.MaxLogRefreshIDs {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ids = append(ids, id)
	}
	logs, err := model.GetLogsByIDs(userID, ids)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, logs)
}
