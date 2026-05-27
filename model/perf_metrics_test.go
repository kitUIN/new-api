package model

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetPerfMetricsSummaryAggregatesBuckets(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "hour",
		FlushInterval: 5,
	})

	now := time.Now().Unix()
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        now - 60,
		ModelName:        "gpt-test",
		Group:            "default",
		Success:          true,
		LatencyMs:        2000,
		TTFTMs:           500,
		CompletionTokens: 120,
		TPSLatencyMs:     2000,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        now - 30,
		ModelName:        "gpt-test",
		Group:            "default",
		Success:          true,
		LatencyMs:        1000,
		TTFTMs:           250,
		CompletionTokens: 60,
		TPSLatencyMs:     1000,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: now - 15,
		ModelName: "gpt-test",
		Group:     "default",
		Success:   false,
		LatencyMs: 1000,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        now - int64(25*time.Hour.Seconds()),
		ModelName:        "old-model",
		Group:            "default",
		Success:          true,
		LatencyMs:        1000,
		CompletionTokens: 100,
		TPSLatencyMs:     1000,
	})
	require.NoError(t, FlushPerfMetrics())

	summary, err := GetPerfMetricsSummary(24, 10)
	require.NoError(t, err)
	require.Len(t, summary.Models, 1)

	modelSummary := summary.Models[0]
	require.Equal(t, "gpt-test", modelSummary.ModelName)
	require.EqualValues(t, 3, modelSummary.RequestCount)
	require.Equal(t, 66.67, modelSummary.SuccessRate)
	require.Equal(t, 1500.0, modelSummary.AvgLatencyMs)
	require.Equal(t, 60.0, modelSummary.AvgTps)
}

func TestGetPerfMetricsSummaryIncludesPendingSamples(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "hour",
		FlushInterval: 5,
	})

	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        time.Now().Unix(),
		ModelName:        "pending-summary",
		Group:            "default",
		Success:          true,
		LatencyMs:        1200,
		CompletionTokens: 24,
		TPSLatencyMs:     1200,
	})

	summary, err := GetPerfMetricsSummary(24, 10)
	require.NoError(t, err)
	require.Len(t, summary.Models, 1)
	require.Equal(t, "pending-summary", summary.Models[0].ModelName)
	require.EqualValues(t, 1, summary.Models[0].RequestCount)
	require.Equal(t, 100.0, summary.Models[0].SuccessRate)
	require.Equal(t, 1200.0, summary.Models[0].AvgLatencyMs)
	require.Equal(t, 20.0, summary.Models[0].AvgTps)
}

func TestGetPerformanceMetricsGroupsAndBuckets(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "hour",
		FlushInterval: 5,
	})

	base := time.Now().Unix()
	base = base - base%3600
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 60,
		ModelName:        "claude-test",
		Group:            "vip",
		Success:          true,
		LatencyMs:        3000,
		TTFTMs:           700,
		CompletionTokens: 90,
		TPSLatencyMs:     3000,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: base + 120,
		ModelName: "claude-test",
		Group:     "vip",
		Success:   false,
		LatencyMs: 1000,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 3660,
		ModelName:        "claude-test",
		Group:            "default",
		Success:          true,
		LatencyMs:        2000,
		TTFTMs:           400,
		CompletionTokens: 80,
		TPSLatencyMs:     2000,
	})
	require.NoError(t, FlushPerfMetrics())

	metrics, err := GetPerformanceMetrics("claude-test", 24)
	require.NoError(t, err)
	require.Equal(t, "claude-test", metrics.ModelName)
	require.Equal(t, "hour", metrics.SeriesSchema)
	require.Len(t, metrics.Groups, 2)

	require.Equal(t, "default", metrics.Groups[0].Group)
	require.Equal(t, 100.0, metrics.Groups[0].SuccessRate)
	require.Equal(t, 2000.0, metrics.Groups[0].AvgLatencyMs)
	require.Equal(t, 400.0, metrics.Groups[0].AvgTTFTMs)
	require.Equal(t, 40.0, metrics.Groups[0].AvgTps)
	require.Len(t, metrics.Groups[0].Series, 1)
	require.Equal(t, base+3600, metrics.Groups[0].Series[0].Ts)

	require.Equal(t, "vip", metrics.Groups[1].Group)
	require.Equal(t, 50.0, metrics.Groups[1].SuccessRate)
	require.Equal(t, 3000.0, metrics.Groups[1].AvgLatencyMs)
	require.Equal(t, 700.0, metrics.Groups[1].AvgTTFTMs)
	require.Equal(t, 30.0, metrics.Groups[1].AvgTps)
	require.Len(t, metrics.Groups[1].Series, 1)
	require.Equal(t, base, metrics.Groups[1].Series[0].Ts)
}

func TestGetPerformanceMetricsIncludesPendingSamples(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "hour",
		FlushInterval: 5,
	})

	base := time.Now().Unix()
	base = base - base%3600
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 30,
		ModelName:        "pending-detail",
		Group:            "vip",
		Success:          true,
		LatencyMs:        1500,
		TTFTMs:           300,
		CompletionTokens: 45,
		TPSLatencyMs:     1500,
	})

	metrics, err := GetPerformanceMetrics("pending-detail", 24)
	require.NoError(t, err)
	require.Len(t, metrics.Groups, 1)
	require.Equal(t, "vip", metrics.Groups[0].Group)
	require.Equal(t, 100.0, metrics.Groups[0].SuccessRate)
	require.Equal(t, 1500.0, metrics.Groups[0].AvgLatencyMs)
	require.Equal(t, 300.0, metrics.Groups[0].AvgTTFTMs)
	require.Equal(t, 30.0, metrics.Groups[0].AvgTps)
	require.Len(t, metrics.Groups[0].Series, 1)
	require.Equal(t, base, metrics.Groups[0].Series[0].Ts)
}

func TestStartPerfMetricsFlushLoopFlushesPendingSamples(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "hour",
		FlushInterval: 5,
	})

	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        time.Now().Unix(),
		ModelName:        "loop-flush",
		Group:            "default",
		Success:          true,
		LatencyMs:        900,
		CompletionTokens: 18,
		TPSLatencyMs:     900,
	})

	sleepCalls := 0
	startPerfMetricsFlushLoop(context.Background(), func(_ context.Context, duration time.Duration) bool {
		sleepCalls++
		require.Equal(t, 5*time.Minute, duration)
		return sleepCalls == 1
	})

	require.Equal(t, 2, sleepCalls)
	var bucket PerfMetricBucket
	require.NoError(t, LOG_DB.Where("model_name = ?", "loop-flush").First(&bucket).Error)
	require.EqualValues(t, 1, bucket.RequestCount)
	require.EqualValues(t, 1, bucket.SuccessCount)
	require.EqualValues(t, 900, bucket.TotalLatencyMs)
}

func TestFlushPerfMetricsIncrementsExistingBucket(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "hour",
		FlushInterval: 5,
	})

	base := time.Now().Unix()
	base = base - base%3600
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 1,
		ModelName:        "merge-test",
		Group:            "default",
		Success:          true,
		LatencyMs:        1000,
		CompletionTokens: 10,
		TPSLatencyMs:     1000,
	})
	require.NoError(t, FlushPerfMetrics())
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 2,
		ModelName:        "merge-test",
		Group:            "default",
		Success:          true,
		LatencyMs:        3000,
		CompletionTokens: 30,
		TPSLatencyMs:     3000,
	})
	require.NoError(t, FlushPerfMetrics())

	var bucket PerfMetricBucket
	require.NoError(t, LOG_DB.Where("model_name = ?", "merge-test").First(&bucket).Error)
	require.EqualValues(t, 2, bucket.RequestCount)
	require.EqualValues(t, 2, bucket.SuccessCount)
	require.EqualValues(t, 4000, bucket.TotalLatencyMs)
	require.EqualValues(t, 40, bucket.CompletionTokens)
	require.EqualValues(t, 4000, bucket.TotalTPSLatencyMs)
}

func TestCleanupPerfMetricsDeletesExpiredBuckets(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create([]PerfMetricBucket{
		{
			BucketStart:   now - int64(48*time.Hour.Seconds()),
			BucketSeconds: 3600,
			ModelName:     "old-model",
			Group:         "default",
			RequestCount:  1,
		},
		{
			BucketStart:   now,
			BucketSeconds: 3600,
			ModelName:     "new-model",
			Group:         "default",
			RequestCount:  1,
		},
	}).Error)

	require.NoError(t, CleanupPerfMetrics(1))

	var count int64
	require.NoError(t, LOG_DB.Model(&PerfMetricBucket{}).Where("model_name = ?", "old-model").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, LOG_DB.Model(&PerfMetricBucket{}).Where("model_name = ?", "new-model").Count(&count).Error)
	require.EqualValues(t, 1, count)
}
