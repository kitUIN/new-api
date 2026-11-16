package controller

import (
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func StreamAllLogs(c *gin.Context)  { streamUsageLogs(c, 0) }
func StreamUserLogs(c *gin.Context) { streamUsageLogs(c, c.GetInt("id")) }

func streamUsageLogs(c *gin.Context, userID int) {
	changes, unsubscribe := model.SubscribeUsageLogs(userID)
	defer unsubscribe()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	// Refresh on every connection, including reconnection after missed events.
	c.SSEvent("changed", "refresh")
	c.Writer.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	// Reconnect periodically so the normal auth middleware revalidates access.
	lifetime := time.NewTimer(5 * time.Minute)
	defer heartbeat.Stop()
	defer lifetime.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-lifetime.C:
			c.SSEvent("reconnect", "refresh")
			c.Writer.Flush()
			return
		case <-changes:
			c.SSEvent("changed", "refresh")
			c.Writer.Flush()
		case <-heartbeat.C:
			c.SSEvent("heartbeat", "keepalive")
			c.Writer.Flush()
		}
	}
}
