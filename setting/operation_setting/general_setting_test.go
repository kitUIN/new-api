package operation_setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetUpstreamFirstResponseTimeout(t *testing.T) {
	setting := GetGeneralSetting()
	originalEnabled := setting.UpstreamFirstResponseTimeoutEnabled
	originalSeconds := setting.UpstreamFirstResponseTimeoutSeconds
	t.Cleanup(func() {
		setting.UpstreamFirstResponseTimeoutEnabled = originalEnabled
		setting.UpstreamFirstResponseTimeoutSeconds = originalSeconds
	})

	setting.UpstreamFirstResponseTimeoutEnabled = false
	require.Zero(t, GetUpstreamFirstResponseTimeout())

	setting.UpstreamFirstResponseTimeoutEnabled = true
	setting.UpstreamFirstResponseTimeoutSeconds = 12
	require.Equal(t, 12*time.Second, GetUpstreamFirstResponseTimeout())

	setting.UpstreamFirstResponseTimeoutSeconds = 0
	require.Equal(t, 30*time.Second, GetUpstreamFirstResponseTimeout())
}
