package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTicketMemoryRateLimitUsesIndependentUserQuotas(t *testing.T) {
	oldRedis := common.RedisEnabled
	oldTicket, oldAttachment := common.TicketRateLimitNum, common.TicketAttachmentRateLimitNum
	common.RedisEnabled = false
	common.TicketRateLimitNum, common.TicketAttachmentRateLimitNum = 2, 2
	t.Cleanup(func() {
		common.RedisEnabled = oldRedis
		common.TicketRateLimitNum, common.TicketAttachmentRateLimitNum = oldTicket, oldAttachment
	})
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		id, _ := strconv.Atoi(c.GetHeader("Test-User"))
		c.Set("id", id)
	})
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	engine.POST("/ticket", TicketRateLimit(), ok)
	engine.POST("/reply", TicketRateLimit(), ok)
	engine.GET("/image", TicketAttachmentRateLimit(), ok)
	request := func(method, path string, user int, ip string) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Test-User", strconv.Itoa(user))
		req.RemoteAddr = ip + ":1234"
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response.Code
	}
	user := int(time.Now().UnixNano())
	require.Equal(t, http.StatusOK, request("POST", "/ticket", user, "192.0.2.1"))
	require.Equal(t, http.StatusOK, request("POST", "/reply", user, "192.0.2.1"))
	require.Equal(t, http.StatusTooManyRequests, request("POST", "/reply", user, "192.0.2.2"))
	require.Equal(t, http.StatusOK, request("POST", "/ticket", user+1, "192.0.2.1"))
	require.Equal(t, http.StatusOK, request("GET", "/image", user, "192.0.2.1"))
	require.Equal(t, http.StatusOK, request("GET", "/image", user, "192.0.2.1"))
	require.Equal(t, http.StatusTooManyRequests, request("GET", "/image", user, "192.0.2.2"))
	require.Equal(t, http.StatusOK, request("GET", "/image", user+1, "192.0.2.1"))
	require.Equal(t, http.StatusUnauthorized, request("GET", "/image", 0, "192.0.2.1"))
}
