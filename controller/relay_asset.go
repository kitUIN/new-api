package controller

import (
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func ServeRelayAsset(c *gin.Context) { service.RelayAssets.ServeHTTP(c.Writer, c.Request) }
func GetRelayAssetStats(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "data": service.RelayAssets.Stats()})
}
