package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestRankingConfigUsesNaturalPeriods(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 6, 10, 15, 30, 0, 0, loc)

	today, err := rankingConfig("today", now, 0, 0)
	require.NoError(t, err)
	require.Equal(t, "today", today.id)
	require.Equal(t, time.Date(2026, 6, 10, 0, 0, 0, 0, loc).Unix(), today.startTime)
	require.Equal(t, now.Unix(), today.endTime)

	yesterday, err := rankingConfig("yesterday", now, 0, 0)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 9, 0, 0, 0, 0, loc).Unix(), yesterday.startTime)
	require.Equal(t, time.Date(2026, 6, 9, 23, 59, 59, 0, loc).Unix(), yesterday.endTime)

	week, err := rankingConfig("week", now, 0, 0)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 8, 0, 0, 0, 0, loc).Unix(), week.startTime)

	lastWeek, err := rankingConfig("last_week", now, 0, 0)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, loc).Unix(), lastWeek.startTime)
	require.Equal(t, time.Date(2026, 6, 7, 23, 59, 59, 0, loc).Unix(), lastWeek.endTime)
	require.Equal(t, time.Date(2026, 5, 25, 0, 0, 0, 0, loc).Unix(), lastWeek.previousStartTime)
	require.Equal(t, time.Date(2026, 5, 31, 23, 59, 59, 0, loc).Unix(), lastWeek.previousEndTime)

	month, err := rankingConfig("month", now, 0, 0)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, loc).Unix(), month.startTime)

	lastMonth, err := rankingConfig("last_month", now, 0, 0)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 5, 1, 0, 0, 0, 0, loc).Unix(), lastMonth.startTime)
	require.Equal(t, time.Date(2026, 5, 31, 23, 59, 59, 0, loc).Unix(), lastMonth.endTime)
	require.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, loc).Unix(), lastMonth.previousStartTime)
	require.Equal(t, time.Date(2026, 4, 30, 23, 59, 59, 0, loc).Unix(), lastMonth.previousEndTime)

	year, err := rankingConfig("year", now, 0, 0)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, loc).Unix(), year.startTime)

	all, err := rankingConfig("all", now, 0, 0)
	require.NoError(t, err)
	require.Zero(t, all.startTime)
	require.Equal(t, now.Unix(), all.endTime)

	customStart := time.Date(2026, 6, 2, 0, 0, 0, 0, loc).Unix()
	customEnd := time.Date(2026, 6, 5, 23, 59, 59, 0, loc).Unix()
	custom, err := rankingConfig("custom", now, customStart, customEnd)
	require.NoError(t, err)
	require.Equal(t, customStart, custom.startTime)
	require.Equal(t, customEnd, custom.endTime)
	require.EqualValues(t, 24*3600, custom.bucketSize)
	require.Equal(t, "Jan 2", custom.labelLayout)
	previousStart, previousEnd := previousRankingTimeRange(custom)
	require.Equal(t, customStart-(customEnd-customStart+1), previousStart)
	require.Equal(t, customStart-1, previousEnd)
}

func TestCustomRankingBucketUsesBoundedGranularity(t *testing.T) {
	tests := []struct {
		duration   int64
		bucketSize int64
		layout     string
	}{
		{duration: 2 * 24 * 3600, bucketSize: 3600, layout: "Jan 2 15:04"},
		{duration: 2*24*3600 + 1, bucketSize: 24 * 3600, layout: "Jan 2"},
		{duration: 90 * 24 * 3600, bucketSize: 24 * 3600, layout: "Jan 2"},
		{duration: 90*24*3600 + 1, bucketSize: 7 * 24 * 3600, layout: "Jan 2"},
		{duration: 2*365*24*3600 + 1, bucketSize: 30 * 24 * 3600, layout: "Jan 2006"},
	}

	for _, test := range tests {
		bucketSize, layout := customRankingBucket(test.duration)
		require.Equal(t, test.bucketSize, bucketSize)
		require.Equal(t, test.layout, layout)
	}
}

func TestRankingConfigRejectsInvalidCustomPeriod(t *testing.T) {
	now := time.Date(2026, 6, 10, 15, 30, 0, 0, time.UTC)

	_, err := rankingConfig("custom", now, 0, 0)
	require.ErrorContains(t, err, "requires start_time and end_time")

	_, err = rankingConfig("custom", now, now.Unix(), now.Add(-time.Hour).Unix())
	require.ErrorContains(t, err, "must not be after end_time")
}

func TestBuildModelHistoryIncludesNaturalPeriodStart(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	tests := []struct {
		name        string
		start       time.Time
		end         time.Time
		dataTime    time.Time
		wantBuckets int
	}{
		{
			name:        "week starts on Monday",
			start:       time.Date(2026, 6, 8, 0, 0, 0, 0, loc),
			end:         time.Date(2026, 6, 10, 15, 30, 0, 0, loc),
			dataTime:    time.Date(2026, 6, 10, 0, 0, 0, 0, loc),
			wantBuckets: 3,
		},
		{
			name:        "month starts on first day",
			start:       time.Date(2026, 6, 1, 0, 0, 0, 0, loc),
			end:         time.Date(2026, 6, 10, 15, 30, 0, 0, loc),
			dataTime:    time.Date(2026, 6, 10, 0, 0, 0, 0, loc),
			wantBuckets: 10,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := rankingPeriodConfig{
				startTime:   test.start.Unix(),
				endTime:     test.end.Unix(),
				bucketSize:  24 * 3600,
				labelLayout: "Jan 2",
			}
			history := buildModelHistory(
				[]model.RankingQuotaBucket{{ModelName: "gpt-a", Bucket: test.dataTime.Unix(), Tokens: 100}},
				[]model.RankingQuotaTotal{{ModelName: "gpt-a", TotalTokens: 100}},
				nil,
				config,
			)

			require.Equal(t, test.wantBuckets, history.Buckets)
			require.Len(t, history.Points, test.wantBuckets)
			require.Equal(t, rankingBucketTs(test.start.Unix()), history.Points[0].Ts)
			require.Zero(t, history.Points[0].Tokens)
			require.EqualValues(t, 100, history.Points[len(history.Points)-1].Tokens)
		})
	}
}
