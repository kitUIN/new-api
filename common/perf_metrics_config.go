package common

import "sync/atomic"

// PerfMetricsConfig controls Relay model performance metrics collection.
type PerfMetricsConfig struct {
	Enabled       bool
	FlushInterval int
	BucketTime    string
	RetentionDays int
	BucketSeconds int64
}

var perfMetricsConfig atomic.Value

func init() {
	perfMetricsConfig.Store(PerfMetricsConfig{
		Enabled:       true,
		FlushInterval: 5,
		BucketTime:    "hour",
		RetentionDays: 0,
		BucketSeconds: 3600,
	})
}

func GetPerfMetricsConfig() PerfMetricsConfig {
	return perfMetricsConfig.Load().(PerfMetricsConfig)
}

func SetPerfMetricsConfig(config PerfMetricsConfig) {
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5
	}
	if config.RetentionDays < 0 {
		config.RetentionDays = 0
	}
	config.BucketSeconds = PerfMetricsBucketSeconds(config.BucketTime)
	config.BucketTime = PerfMetricsBucketName(config.BucketTime)
	perfMetricsConfig.Store(config)
}

func PerfMetricsBucketSeconds(bucketTime string) int64 {
	switch bucketTime {
	case "minute":
		return 60
	case "5min":
		return 300
	case "hour":
		return 3600
	default:
		return 3600
	}
}

func PerfMetricsBucketName(bucketTime string) string {
	switch bucketTime {
	case "minute", "5min", "hour":
		return bucketTime
	default:
		return "hour"
	}
}
