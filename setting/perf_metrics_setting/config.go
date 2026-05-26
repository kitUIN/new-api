package perf_metrics_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type PerfMetricsSetting struct {
	Enabled       bool   `json:"enabled"`
	FlushInterval int    `json:"flush_interval"`
	BucketTime    string `json:"bucket_time"`
	RetentionDays int    `json:"retention_days"`
}

var perfMetricsSetting = PerfMetricsSetting{
	Enabled:       true,
	FlushInterval: 5,
	BucketTime:    "hour",
	RetentionDays: 0,
}

func init() {
	config.GlobalConfig.Register("perf_metrics_setting", &perfMetricsSetting)
	syncToCommon()
}

func syncToCommon() {
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       perfMetricsSetting.Enabled,
		FlushInterval: perfMetricsSetting.FlushInterval,
		BucketTime:    perfMetricsSetting.BucketTime,
		RetentionDays: perfMetricsSetting.RetentionDays,
	})
}

func GetPerfMetricsSetting() *PerfMetricsSetting {
	return &perfMetricsSetting
}

func UpdateAndSync() {
	syncToCommon()
}
