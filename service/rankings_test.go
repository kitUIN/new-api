package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRankingConfigUsesNaturalPeriods(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 6, 10, 15, 30, 0, 0, loc)

	today, err := rankingConfig("today", now)
	require.NoError(t, err)
	require.Equal(t, "today", today.id)
	require.Equal(t, time.Date(2026, 6, 10, 0, 0, 0, 0, loc).Unix(), today.startTime)
	require.Equal(t, now.Unix(), today.endTime)

	yesterday, err := rankingConfig("yesterday", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 9, 0, 0, 0, 0, loc).Unix(), yesterday.startTime)
	require.Equal(t, time.Date(2026, 6, 9, 23, 59, 59, 0, loc).Unix(), yesterday.endTime)

	week, err := rankingConfig("week", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 8, 0, 0, 0, 0, loc).Unix(), week.startTime)

	month, err := rankingConfig("month", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, loc).Unix(), month.startTime)

	year, err := rankingConfig("year", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, loc).Unix(), year.startTime)

	all, err := rankingConfig("all", now)
	require.NoError(t, err)
	require.Zero(t, all.startTime)
	require.Equal(t, now.Unix(), all.endTime)
}
