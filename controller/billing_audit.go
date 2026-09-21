package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetBillingAudit(c *gin.Context) {
	month, _, _, err := service.BillingAuditMonth(c.Query("month"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	result, err := service.GetBillingAuditSummary(month)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetBillingAuditTopUps(c *gin.Context) {
	_, start, end, err := service.BillingAuditMonth(c.Query("month"))
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || pageErr != nil || sizeErr != nil || page < 1 || page > 1000000 || size < 1 || size > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid month or pagination"})
		return
	}
	rows, total, err := model.GetBillingTopUps(start, end, page, size)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": rows, "total": total}})
}

func SaveBillingAuditCost(c *gin.Context) {
	id := 0
	if c.Param("id") != "" {
		parsed, err := strconv.Atoi(c.Param("id"))
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid cost id"})
			return
		}
		id = parsed
	}
	var input service.BillingCostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid cost input"})
		return
	}
	if err := service.SaveBillingCost(id, c.GetInt("id"), input, c.Request.Method == http.MethodDelete); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
