package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

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

func TestAppendRelayTransportInfoMarksOnlyWebSocket(t *testing.T) {
	webSocketOther := map[string]interface{}{}
	AppendRelayTransportInfo(&relaycommon.RelayInfo{
		Transport: types.RelayTransportWebSocket,
	}, webSocketOther)
	require.Equal(t, true, webSocketOther["ws"])

	httpOther := map[string]interface{}{}
	AppendRelayTransportInfo(&relaycommon.RelayInfo{
		Transport: types.RelayTransportHTTP,
	}, httpOther)
	require.NotContains(t, httpOther, "ws")
}

func TestAppendRelayTimingSplitsPreparation(t *testing.T) {
	start := time.Now().Add(-time.Second)
	for _, tc := range []struct {
		name      string
		received  time.Time
		receiveMs int64
		prepareMs int64
		split     bool
	}{
		{"normal", start.Add(40 * time.Millisecond), 40, 60, true},
		{"zero", start, 0, 100, true},
		{"legacy", time.Time{}, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := map[string]interface{}{}
			AppendRelayTimingInfo(nil, &relaycommon.RelayInfo{StartTime: start, RequestBodyReceivedTime: tc.received, UpstreamRequestStartTime: start.Add(100 * time.Millisecond)}, other)
			require.Equal(t, int64(100), other["pre_upstream_ms"])
			if tc.split {
				require.Equal(t, tc.receiveMs, other["request_body_receive_ms"])
				require.Equal(t, tc.prepareMs, other["upstream_prepare_ms"])
				require.Equal(t, formatRelayTiming(tc.received), other["request_body_received_at"])
			} else {
				require.NotContains(t, other, "request_body_receive_ms")
				require.NotContains(t, other, "upstream_prepare_ms")
			}
		})
	}
}
