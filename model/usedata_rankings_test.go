package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRankingUserTotalsAndSelfRank(t *testing.T) {
	truncateTables(t)

	base := int64(1710000000)
	rows := []QuotaData{
		{UserID: 3, Username: "user3", ModelName: "gpt-a", Group: "default", CreatedAt: base, TokenUsed: 500, Quota: 1000},
		{UserID: 1, Username: "user1", ModelName: "gpt-a", Group: "default", CreatedAt: base, TokenUsed: 700, Quota: 1200},
		{UserID: 2, Username: "user2", ModelName: "gpt-b", Group: "vip", CreatedAt: base, TokenUsed: 700, Quota: 1300},
		{UserID: 4, Username: "user4", ModelName: "gpt-c", Group: "vip", CreatedAt: base, TokenUsed: 0, Quota: 999},
	}
	for _, row := range rows {
		require.NoError(t, DB.Create(&row).Error)
	}

	totals, err := GetRankingUserTotals(base-1, base+1, 5)
	require.NoError(t, err)
	require.Len(t, totals, 3)
	require.Equal(t, 1, totals[0].UserID)
	require.EqualValues(t, 700, totals[0].TotalTokens)
	require.Equal(t, 2, totals[1].UserID)
	require.EqualValues(t, 700, totals[1].TotalTokens)
	require.Equal(t, 3, totals[2].UserID)
	require.EqualValues(t, 500, totals[2].TotalTokens)

	quotaTotals, err := GetRankingUserTotalsByMetric(base-1, base+1, 5, RankingUserMetricQuota)
	require.NoError(t, err)
	require.Len(t, quotaTotals, 4)
	require.Equal(t, 2, quotaTotals[0].UserID)
	require.EqualValues(t, 1300, quotaTotals[0].TotalQuota)
	require.Equal(t, 1, quotaTotals[1].UserID)
	require.EqualValues(t, 1200, quotaTotals[1].TotalQuota)

	self, err := GetRankingUserTotal(2, base-1, base+1)
	require.NoError(t, err)
	require.EqualValues(t, 700, self.TotalTokens)

	rank, err := GetRankingUserRank(2, self.TotalTokens, base-1, base+1)
	require.NoError(t, err)
	require.Equal(t, 2, rank)
	quotaRank, err := GetRankingUserRankByMetric(2, self.TotalQuota, base-1, base+1, RankingUserMetricQuota)
	require.NoError(t, err)
	require.Equal(t, 1, quotaRank)

	empty, err := GetRankingUserTotal(99, base-1, base+1)
	require.NoError(t, err)
	require.Equal(t, 99, empty.UserID)
	require.Zero(t, empty.TotalTokens)
	emptyRank, err := GetRankingUserRank(99, empty.TotalTokens, base-1, base+1)
	require.NoError(t, err)
	require.Zero(t, emptyRank)
}

func TestRankingGroupTotalsNormalizeAndLimit(t *testing.T) {
	truncateTables(t)

	base := int64(1710000000)
	rows := []QuotaData{
		{UserID: 1, ModelName: "gpt-a", Group: "", CreatedAt: base, TokenUsed: 100, Quota: 200},
		{UserID: 2, ModelName: "gpt-b", Group: "default", CreatedAt: base, TokenUsed: 200, Quota: 300},
		{UserID: 3, ModelName: "gpt-c", Group: "vip", CreatedAt: base, TokenUsed: 500, Quota: 700},
		{UserID: 4, ModelName: "gpt-d", Group: "svip", CreatedAt: base, TokenUsed: 300, Quota: 600},
	}
	for _, row := range rows {
		require.NoError(t, DB.Create(&row).Error)
	}

	totals, err := GetRankingGroupTotals(base-1, base+1, 2)
	require.NoError(t, err)
	require.Len(t, totals, 2)
	require.Equal(t, "vip", totals[0].Group)
	require.EqualValues(t, 500, totals[0].TotalTokens)
	require.Equal(t, "default", totals[1].Group)
	require.EqualValues(t, 300, totals[1].TotalTokens)
}

func TestRankingQuotaBucketsAlignToRangeStart(t *testing.T) {
	truncateTables(t)

	loc := time.FixedZone("CST", 8*3600)
	dayStart := time.Date(2026, 8, 25, 0, 0, 0, 0, loc).Unix()
	rows := []QuotaData{
		{UserID: 1, ModelName: "gpt-a", Group: "default", CreatedAt: dayStart + 3600, TokenUsed: 100},
		{UserID: 1, ModelName: "gpt-a", Group: "default", CreatedAt: dayStart + 9*3600, TokenUsed: 200},
		{UserID: 1, ModelName: "gpt-a", Group: "default", CreatedAt: dayStart + 25*3600, TokenUsed: 300},
	}
	for _, row := range rows {
		require.NoError(t, DB.Create(&row).Error)
	}

	buckets, err := GetRankingQuotaBuckets(dayStart, dayStart+2*24*3600-1, 24*3600)
	require.NoError(t, err)
	require.Equal(t, []RankingQuotaBucket{
		{ModelName: "gpt-a", Bucket: dayStart, Tokens: 300},
		{ModelName: "gpt-a", Bucket: dayStart + 24*3600, Tokens: 300},
	}, buckets)
}

func TestRankingGroupPerfStatsIncludesFlushedAndPending(t *testing.T) {
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
		ModelName:        "gpt-a",
		Group:            "vip",
		Success:          true,
		LatencyMs:        2000,
		TTFTMs:           300,
		CompletionTokens: 100,
		TPSLatencyMs:     2000,
	})
	RecordPerfMetricSample(PerfMetricSample{
		Timestamp: base + 120,
		ModelName: "gpt-a",
		Group:     "vip",
		Success:   false,
	})
	require.NoError(t, FlushPerfMetrics())

	RecordPerfMetricSample(PerfMetricSample{
		Timestamp:        base + 180,
		ModelName:        "gpt-b",
		Group:            "vip",
		Success:          true,
		LatencyMs:        1000,
		TTFTMs:           500,
		CompletionTokens: 50,
		TPSLatencyMs:     1000,
	})

	stats, err := GetRankingGroupPerfStats(base, base+3600)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, "vip", stats[0].Group)
	require.EqualValues(t, 3, stats[0].RequestCount)
	require.Equal(t, 66.67, stats[0].SuccessRate)
	require.Equal(t, 400.0, stats[0].AvgTTFTMs)
	require.Equal(t, 1500.0, stats[0].AvgLatencyMs)
	require.Equal(t, 50.0, stats[0].AvgTps)
}
