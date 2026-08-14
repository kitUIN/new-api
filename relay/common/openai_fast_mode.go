package common

import (
	"strings"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

const OpenAIFastModeBillingMultiplier = 2.5

func normalizeServiceTier(tier string) string {
	return strings.ToLower(strings.TrimSpace(tier))
}

func IsOpenAIFastServiceTier(tier string) bool {
	switch normalizeServiceTier(tier) {
	case "fast", "priority":
		return true
	default:
		return false
	}
}

func serviceTierFromRequest(request dto.Request) string {
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if req == nil || len(req.ServiceTier) == 0 {
			return ""
		}
		var tier string
		if err := rootcommon.Unmarshal(req.ServiceTier, &tier); err != nil {
			return ""
		}
		return normalizeServiceTier(tier)
	case *dto.OpenAIResponsesRequest:
		if req == nil {
			return ""
		}
		return normalizeServiceTier(req.ServiceTier)
	default:
		return ""
	}
}

// RefreshOpenAIFastModeBilling keeps billing aligned with the request that can
// actually reach upstream. A filtered service_tier must not affect billing.
func (info *RelayInfo) RefreshOpenAIFastModeBilling(c *gin.Context) {
	if info == nil {
		return
	}

	info.ServiceTierPassthroughEnabled = false
	info.ServiceTierBillingMultiplier = 1
	if c == nil {
		return
	}

	channelSetting, _ := rootcommon.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
	channelOtherSetting, _ := rootcommon.GetContextKeyType[dto.ChannelOtherSettings](c, constant.ContextKeyChannelOtherSetting)
	info.ServiceTierPassthroughEnabled = model_setting.GetGlobalSettings().PassThroughRequestEnabled ||
		channelSetting.PassThroughBodyEnabled || channelOtherSetting.AllowServiceTier

	channelType := rootcommon.GetContextKeyInt(c, constant.ContextKeyChannelType)
	if channelType == constant.ChannelTypeOpenAI &&
		info.ServiceTierPassthroughEnabled &&
		IsOpenAIFastServiceTier(info.ServiceTier) {
		info.ServiceTierBillingMultiplier = OpenAIFastModeBillingMultiplier
	}
}

func (info *RelayInfo) GetServiceTierBillingMultiplier() float64 {
	if info == nil || info.ServiceTierBillingMultiplier <= 0 {
		return 1
	}
	return info.ServiceTierBillingMultiplier
}

func (info *RelayInfo) ShouldStripServiceTierFromBillingInput() bool {
	if info == nil || info.ServiceTier == "" {
		return false
	}
	return !info.ServiceTierPassthroughEnabled || info.GetServiceTierBillingMultiplier() > 1
}
