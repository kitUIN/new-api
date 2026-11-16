package service

import (
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func BeginRelayLog(c *gin.Context, info *relaycommon.RelayInfo) {
	other := make(map[string]interface{})
	appendRequestPath(c, info, other)
	AppendRelayTransportInfo(info, other)
	admin := make(map[string]interface{})
	AppendChannelAffinitySessionKeyAdminInfo(c, admin)
	other["admin_info"] = admin
	model.BeginRequestLog(c, info, other)
}
