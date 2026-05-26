package perf_metrics_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestPerfMetricsSettingExportsDefaults(t *testing.T) {
	configs := config.GlobalConfig.ExportAllConfigs()

	require.Equal(t, "true", configs["perf_metrics_setting.enabled"])
	require.Equal(t, "5", configs["perf_metrics_setting.flush_interval"])
	require.Equal(t, "hour", configs["perf_metrics_setting.bucket_time"])
	require.Equal(t, "0", configs["perf_metrics_setting.retention_days"])
}

func TestPerfMetricsSettingSyncsToCommon(t *testing.T) {
	original := *GetPerfMetricsSetting()
	t.Cleanup(func() {
		perfMetricsSetting = original
		UpdateAndSync()
	})

	perfMetricsSetting.Enabled = false
	perfMetricsSetting.FlushInterval = 2
	perfMetricsSetting.BucketTime = "5min"
	perfMetricsSetting.RetentionDays = 7
	UpdateAndSync()

	runtimeConfig := common.GetPerfMetricsConfig()
	require.False(t, runtimeConfig.Enabled)
	require.Equal(t, 2, runtimeConfig.FlushInterval)
	require.Equal(t, "5min", runtimeConfig.BucketTime)
	require.EqualValues(t, 300, runtimeConfig.BucketSeconds)
	require.Equal(t, 7, runtimeConfig.RetentionDays)
}
