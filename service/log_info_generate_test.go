package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

func TestAppendBillingInfoIncludesOpenAIFastModeMetadata(t *testing.T) {
	other := map[string]interface{}{}
	appendBillingInfo(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
		ServiceTier:                  "priority",
		ServiceTierBillingMultiplier: relaycommon.OpenAIFastModeBillingMultiplier,
	}, other)

	require.Equal(t, "fast", other["service_tier"])
	require.Equal(t, relaycommon.OpenAIFastModeBillingMultiplier, other["service_tier_multiplier"])
}

func TestAppendBillingInfoIncludesOpenAIDefaultServiceTierMetadata(t *testing.T) {
	other := map[string]interface{}{}
	appendBillingInfo(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
		ServiceTierBillingMultiplier: 1,
	}, other)

	require.Equal(t, "default", other["service_tier"])
	require.Equal(t, float64(1), other["service_tier_multiplier"])
}

func TestAppendBillingInfoOmitsServiceTierMetadataForOtherChannels(t *testing.T) {
	other := map[string]interface{}{}
	appendBillingInfo(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeAzure,
		},
		ServiceTier:                  "fast",
		ServiceTierBillingMultiplier: relaycommon.OpenAIFastModeBillingMultiplier,
	}, other)

	require.NotContains(t, other, "service_tier")
	require.NotContains(t, other, "service_tier_multiplier")
}
