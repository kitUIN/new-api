package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestRealtimeRequestURLDoesNotMutateChannelBaseURL(t *testing.T) {
	t.Parallel()
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeRealtime,
		RequestURLPath: "/v1/realtime?model=gpt-realtime",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeOpenAI,
			ChannelBaseUrl: "https://api.example.com",
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/v1/realtime?model=gpt-realtime", requestURL)
	require.Equal(t, "https://api.example.com", info.ChannelBaseUrl)
}
