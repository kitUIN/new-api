package model

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestGetGroupRatioHistorySummary(t *testing.T) {
	truncateTables(t)
	original := ratio_setting.GroupRatio2JSONString()
	defer func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(original))
	}()

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	require.NoError(t, RecordGroupRatioChanges(
		map[string]float64{"default": 1, "vip": 1},
		map[string]float64{"default": 1, "vip": 0.5},
		GroupRatioHistorySourceManual,
	))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":0.5}`))

	endTs := time.Now().Unix()
	startTs := endTs - 7*24*60*60
	summary, err := GetGroupRatioHistorySummary(startTs, endTs)
	require.NoError(t, err)
	require.NotEmpty(t, summary.Groups)

	var vipSeries []GroupRatioHistoryPoint
	for _, series := range summary.Groups {
		if series.Group == "vip" {
			vipSeries = series.Points
			break
		}
	}
	require.GreaterOrEqual(t, len(vipSeries), 2)
	require.Equal(t, 1.0, vipSeries[0].Ratio)
	require.Equal(t, 0.5, vipSeries[len(vipSeries)-1].Ratio)
}

func TestRecordGroupRatioChangesSkipsAddedGroups(t *testing.T) {
	truncateTables(t)
	original := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(original))
	})

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"new":0.7}`))
	require.NoError(t, RecordGroupRatioChanges(
		map[string]float64{"default": 1},
		map[string]float64{"default": 1, "new": 0.7},
		GroupRatioHistorySourceManual,
	))

	var count int64
	require.NoError(t, DB.Model(&GroupRatioHistory{}).Where(commonGroupCol+" = ?", "new").Count(&count).Error)
	require.Zero(t, count)

	endTs := time.Now().Unix()
	startTs := endTs - 7*24*60*60
	summary, err := GetGroupRatioHistorySummary(startTs, endTs)
	require.NoError(t, err)

	var newSeries []GroupRatioHistoryPoint
	for _, series := range summary.Groups {
		if series.Group == "new" {
			newSeries = series.Points
			break
		}
	}
	require.Len(t, newSeries, 1)
	require.Equal(t, 0.7, newSeries[0].Ratio)
}

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

func TestPerfMetricSampleCanSkipLatencyMetrics(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "10min",
		FlushInterval: 5,
	})
	insertEnabledChannel(t, 1, "default")

	now := time.Now().Unix()
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        now,
		ModelName:        "skip-latency",
		Group:            "default",
		Success:          true,
		LatencyMs:        1000,
		TTFTMs:           200,
		CompletionTokens: 20,
		TPSLatencyMs:     1000,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:          now,
		ModelName:          "skip-latency",
		Group:              "default",
		Success:            true,
		LatencyMs:          9000,
		TTFTMs:             3000,
		CompletionTokens:   900,
		TPSLatencyMs:       9000,
		SkipLatencyMetrics: true,
	})
	require.NoError(t, FlushPerfMetrics())

	summary, err := GetPerfMetricsSummary(24, 10)
	require.NoError(t, err)
	require.Len(t, summary.Models, 1)
	require.EqualValues(t, 2, summary.Models[0].RequestCount)
	require.Equal(t, 100.0, summary.Models[0].SuccessRate)
	require.Equal(t, 1000.0, summary.Models[0].AvgLatencyMs)
	require.Equal(t, 20.0, summary.Models[0].AvgTps)

	groupSummary, err := GetPerfGroupHealthSummary(1, 10)
	require.NoError(t, err)
	defaultGroup := requirePerfGroupHealth(t, groupSummary.Groups, "default")
	require.EqualValues(t, 2, defaultGroup.RequestCount)
	require.Equal(t, 100.0, defaultGroup.SuccessRate)
	require.Equal(t, 1000.0, defaultGroup.AvgLatencyMs)
	require.Equal(t, 200.0, defaultGroup.AvgTTFTMs)
	require.Equal(t, 20.0, defaultGroup.AvgTps)

	var bucket PerfMetricBucket
	require.NoError(t, LOG_DB.Where("model_name = ?", "skip-latency").First(&bucket).Error)
	require.EqualValues(t, 2, bucket.RequestCount)
	require.EqualValues(t, 2, bucket.SuccessCount)
	require.EqualValues(t, 1, bucket.LatencyCount)
	require.EqualValues(t, 1000, bucket.TotalLatencyMs)
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

func TestGetPerfGroupHealthSummaryAggregatesGroupsAndTenMinuteBuckets(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":0}`))
	insertEnabledChannel(t, 1, "default,vip")
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "10min",
		FlushInterval: 5,
	})

	base := time.Now().Unix()
	base = base - base%600
	previousBase := base - 600
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        previousBase + 10,
		ModelName:        "model-a",
		Group:            "default",
		Success:          true,
		LatencyMs:        1000,
		TTFTMs:           200,
		CompletionTokens: 50,
		TPSLatencyMs:     1000,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: previousBase + 20,
		ModelName: "model-b",
		Group:     "default",
		Success:   false,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 10,
		ModelName:        "model-c",
		Group:            "vip",
		Success:          true,
		LatencyMs:        2000,
		TTFTMs:           400,
		CompletionTokens: 80,
		TPSLatencyMs:     2000,
	})
	require.NoError(t, FlushPerfMetrics())

	summary, err := GetPerfGroupHealthSummary(24, 10)
	require.NoError(t, err)
	require.Equal(t, 24, summary.WindowHours)
	require.Equal(t, 10, summary.IntervalMinutes)
	require.Equal(t, 144, summary.BucketCount)

	defaultGroup := requirePerfGroupHealth(t, summary.Groups, "default")
	require.Equal(t, 1.0, defaultGroup.Ratio)
	require.EqualValues(t, 2, defaultGroup.RequestCount)
	require.Equal(t, 50.0, defaultGroup.SuccessRate)
	require.Equal(t, 1000.0, defaultGroup.AvgLatencyMs)
	require.Equal(t, 200.0, defaultGroup.AvgTTFTMs)
	require.Equal(t, 50.0, defaultGroup.AvgTps)
	require.Len(t, defaultGroup.Buckets, 144)
	defaultBucket := requirePerfGroupHealthBucket(t, defaultGroup.Buckets, previousBase)
	require.EqualValues(t, 2, defaultBucket.RequestCount)
	require.EqualValues(t, 1, defaultBucket.SuccessCount)
	require.Equal(t, 50.0, defaultBucket.SuccessRate)
	require.Equal(t, "error", defaultBucket.Status)

	vipGroup := requirePerfGroupHealth(t, summary.Groups, "vip")
	require.Equal(t, 0.0, vipGroup.Ratio)
	require.EqualValues(t, 1, vipGroup.RequestCount)
	require.Equal(t, 100.0, vipGroup.SuccessRate)
	vipBucket := requirePerfGroupHealthBucket(t, vipGroup.Buckets, base)
	require.Equal(t, "ok", vipBucket.Status)
}

func TestGetPerfGroupHealthSummaryIncludesPendingSamples(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "5min",
		FlushInterval: 5,
	})
	insertEnabledChannel(t, 1, "default")

	base := time.Now().Unix()
	base = base - base%600
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 300,
		ModelName:        "pending-group-health",
		Group:            "default",
		Success:          true,
		LatencyMs:        1200,
		TTFTMs:           250,
		CompletionTokens: 24,
		TPSLatencyMs:     1200,
	})

	summary, err := GetPerfGroupHealthSummary(24, 10)
	require.NoError(t, err)
	defaultGroup := requirePerfGroupHealth(t, summary.Groups, "default")
	require.EqualValues(t, 1, defaultGroup.RequestCount)
	require.Equal(t, 100.0, defaultGroup.SuccessRate)
	bucket := requirePerfGroupHealthBucket(t, defaultGroup.Buckets, base)
	require.Equal(t, "ok", bucket.Status)
}

func TestGetPerfGroupHealthSummaryUsesRecentInMemorySamples(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "10min",
		FlushInterval: 5,
	})
	insertEnabledChannel(t, 1, "default")
	insertEnabledChannel(t, 2, "vip")

	now := time.Now().Unix()
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: now - 12*60,
		ModelName: "recent-fallback-old",
		Group:     "default",
		Success:   true,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: now - 2*60,
		ModelName: "recent-direct-success",
		Group:     "vip",
		Success:   true,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: now - 1*60,
		ModelName: "recent-direct-failure",
		Group:     "vip",
		Success:   false,
	})
	require.NoError(t, FlushPerfMetrics())

	summary, err := GetPerfGroupHealthSummary(24, 10)
	require.NoError(t, err)

	defaultGroup := requirePerfGroupHealth(t, summary.Groups, "default")
	require.EqualValues(t, 1, defaultGroup.RecentRequestCount)
	require.Equal(t, 100.0, defaultGroup.RecentSuccessRate)
	require.Equal(t, 20, defaultGroup.RecentWindowMinutes)

	vipGroup := requirePerfGroupHealth(t, summary.Groups, "vip")
	require.EqualValues(t, 2, vipGroup.RecentRequestCount)
	require.Equal(t, 50.0, vipGroup.RecentSuccessRate)
	require.Equal(t, 10, vipGroup.RecentWindowMinutes)
}

func TestGetPerfGroupHealthSummaryOnlyIncludesEnabledGroups(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"disabled":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default group"}`))
	insertEnabledChannel(t, 1, "default,disabled")
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "10min",
		FlushInterval: 5,
	})

	base := time.Now().Unix()
	base = base - base%600
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 10,
		ModelName:        "enabled-group-health",
		Group:            "default",
		Success:          true,
		LatencyMs:        1200,
		TTFTMs:           250,
		CompletionTokens: 24,
		TPSLatencyMs:     1200,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 20,
		ModelName:        "disabled-group-health",
		Group:            "disabled",
		Success:          true,
		LatencyMs:        800,
		TTFTMs:           180,
		CompletionTokens: 32,
		TPSLatencyMs:     800,
	})
	require.NoError(t, FlushPerfMetrics())

	summary, err := GetPerfGroupHealthSummary(24, 10)
	require.NoError(t, err)
	requirePerfGroupHealth(t, summary.Groups, "default")
	for _, group := range summary.Groups {
		require.NotEqual(t, "disabled", group.Group)
	}
}

func TestGetPerfGroupHealthSummaryFiltersAndSortsGroups(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	monitorSetting := operation_setting.GetMonitorSetting()
	originalSkipGroups := monitorSetting.AutoTestChannelSkipGroups
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		monitorSetting.AutoTestChannelSkipGroups = originalSkipGroups
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2,"cheap":0.5,"skipped":1,"empty":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default group","vip":"VIP","cheap":"Cheap","skipped":"Skipped","empty":"Empty"}`))
	monitorSetting.AutoTestChannelSkipGroups = "skipped"
	insertEnabledChannel(t, 1, "default")
	insertEnabledChannel(t, 2, "vip")
	insertEnabledChannel(t, 3, "cheap")
	insertEnabledChannel(t, 4, "skipped")
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "10min",
		FlushInterval: 5,
	})

	base := time.Now().Unix()
	base = base - base%600
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: base + 10,
		ModelName: "default-low",
		Group:     "default",
		Success:   true,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: base + 20,
		ModelName: "default-low",
		Group:     "default",
		Success:   false,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: base + 30,
		ModelName: "vip-high",
		Group:     "vip",
		Success:   true,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: base + 40,
		ModelName: "cheap-high",
		Group:     "cheap",
		Success:   true,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: base + 50,
		ModelName: "skipped-hidden",
		Group:     "skipped",
		Success:   true,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: base + 60,
		ModelName: "empty-hidden",
		Group:     "empty",
		Success:   true,
	})
	require.NoError(t, FlushPerfMetrics())

	summary, err := GetPerfGroupHealthSummary(24, 10)
	require.NoError(t, err)
	require.Len(t, summary.Groups, 3)
	require.Equal(t, "cheap", summary.Groups[0].Group)
	require.Equal(t, "vip", summary.Groups[1].Group)
	require.Equal(t, "default", summary.Groups[2].Group)
	for _, group := range summary.Groups {
		require.NotEqual(t, "skipped", group.Group)
		require.NotEqual(t, "empty", group.Group)
		require.Greater(t, group.ProviderCount, 0)
	}
}

func TestGetGroupProviderCounts(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create([]Channel{
		{Id: 1, ProviderID: 10, Status: common.ChannelStatusEnabled, Group: "default,vip"},
		{Id: 2, ProviderID: 10, Status: common.ChannelStatusEnabled, Group: "default"},
		{Id: 3, ProviderID: 0, Status: common.ChannelStatusEnabled, Group: "default"},
		{Id: 4, ProviderID: 11, Status: common.ChannelStatusManuallyDisabled, Group: "default"},
	}).Error)

	counts, err := GetGroupProviderCounts()
	require.NoError(t, err)
	require.Equal(t, 2, counts["default"])
	require.Equal(t, 1, counts["vip"])
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

func requirePerfGroupHealth(t *testing.T, groups []PerfGroupHealth, groupName string) PerfGroupHealth {
	t.Helper()
	for _, group := range groups {
		if group.Group == groupName {
			return group
		}
	}
	t.Fatalf("group %q not found", groupName)
	return PerfGroupHealth{}
}

func requirePerfGroupHealthBucket(t *testing.T, buckets []PerfGroupHealthBucket, ts int64) PerfGroupHealthBucket {
	t.Helper()
	for _, bucket := range buckets {
		if bucket.Ts == ts {
			return bucket
		}
	}
	t.Fatalf("bucket %d not found", ts)
	return PerfGroupHealthBucket{}
}

func insertEnabledChannel(t *testing.T, id int, group string) {
	t.Helper()
	require.NoError(t, DB.Create(&Channel{
		Id:     id,
		Status: common.ChannelStatusEnabled,
		Key:    "test-key",
		Group:  group,
	}).Error)
}
