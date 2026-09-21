package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRankingUserPresentationAdminPrivacy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.QuotaData{}))
	users := []model.User{
		{Id: 1, Username: "private", DisplayName: "Private User", QQId: "12345", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Id: 2, Username: "admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled},
		{Id: 3, Username: "root", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 4, Username: "disabled-admin", Role: common.RoleAdminUser, Status: common.UserStatusDisabled},
		{Id: 5, Username: "public", Setting: `{"ranking_public":true}`, Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}
	for i := range users {
		users[i].AffCode = users[i].Username
	}
	require.NoError(t, db.Create(&users).Error)
	data := &RankingsResponse{Users: buildRankedUsers([]model.RankingUserTotal{
		{UserID: 1, TotalTokens: 100, TotalQuota: 200},
		{UserID: 5, TotalTokens: 50, TotalQuota: 100},
	})}
	originalRows := append([]RankedUser(nil), data.Users...)
	for _, test := range []struct {
		name     string
		viewer   int
		wantReal bool
	}{
		{name: "admin", viewer: 2, wantReal: true},
		{name: "guest after admin"},
		{name: "root", viewer: 3, wantReal: true},
		{name: "ordinary user after root", viewer: 1},
		{name: "disabled admin", viewer: 4},
		{name: "missing user", viewer: 99},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := withRankingUserPresentation(data, rankingPeriodConfig{}, test.viewer, RankingUserMetricTokens)
			require.NoError(t, err)
			if test.wantReal {
				require.Equal(t, "Private User", result.Users[0].DisplayName)
				require.Equal(t, rankingQQAvatarURL("12345"), result.Users[0].AvatarURL)
			} else {
				require.Equal(t, "匿名用户1", result.Users[0].DisplayName)
				require.Empty(t, result.Users[0].AvatarURL)
			}
			require.False(t, result.Users[0].RankingPublic)
			require.Equal(t, "public", result.Users[1].DisplayName)
			require.True(t, result.Users[1].RankingPublic)
			require.Equal(t, originalRows, data.Users)
			if test.viewer == 1 {
				require.Equal(t, "Private User", result.SelfUser.DisplayName)
				require.False(t, result.SelfUser.RankingPublic)
			}
		})
	}
}

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
