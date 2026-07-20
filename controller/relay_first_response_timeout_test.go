package controller

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestNormalizeUpstreamFirstResponseTimeout(t *testing.T) {
	err := types.NewError(relaycommon.ErrUpstreamFirstResponseTimeout, types.ErrorCodeDoRequestFailed)

	normalized := normalizeUpstreamFirstResponseTimeout(err, nil)

	require.NotNil(t, normalized)
	require.Equal(t, 504, normalized.StatusCode)
	require.Equal(t, types.ErrorCodeUpstreamFirstResponseTimeout, normalized.GetErrorCode())
	require.ErrorIs(t, normalized, relaycommon.ErrUpstreamFirstResponseTimeout)
}

func TestNormalizeUpstreamFirstResponseTimeoutFromStreamStatus(t *testing.T) {
	info := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}
	info.StreamStatus.SetEndReason(
		relaycommon.StreamEndReasonFirstResponseTimeout,
		relaycommon.ErrUpstreamFirstResponseTimeout,
	)

	normalized := normalizeUpstreamFirstResponseTimeout(nil, info)

	require.NotNil(t, normalized)
	require.Equal(t, 504, normalized.StatusCode)
	require.Equal(t, types.ErrorCodeUpstreamFirstResponseTimeout, normalized.GetErrorCode())
}
