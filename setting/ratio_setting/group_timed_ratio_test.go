package ratio_setting

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestTimedRatioWindows(t *testing.T) {
	for _, tc := range []struct {
		name, kind, start, end, now string
		days                        []int
		active                      bool
		duration                    time.Duration
	}{
		{"once start", "once", "2026-10-13T09:00", "2026-10-13T10:00", "2026-10-13T09:00", nil, true, time.Hour},
		{"once end", "once", "2026-10-13T09:00", "2026-10-13T10:00", "2026-10-13T10:00", nil, false, time.Hour},
		{"once expired", "once", "2026-10-13T09:00", "2026-10-13T10:00", "2026-10-14T09:30", nil, false, time.Hour},
		{"hourly", "hourly", "00:15", "00:45", "2026-10-13T12:15", nil, true, 30 * time.Minute},
		{"hourly end", "hourly", "00:15", "00:45", "2026-10-13T12:45", nil, false, 30 * time.Minute},
		{"hourly wrap", "hourly", "00:45", "00:15", "2026-10-13T00:05", nil, true, 30 * time.Minute},
		{"hourly full", "hourly", "00:00", "01:00", "2026-10-13T12:59", nil, true, time.Hour},
		{"daily wrap", "daily", "23:00", "02:00", "2026-10-13T01:30", nil, true, 3 * time.Hour},
		{"daily outside", "daily", "23:00", "02:00", "2026-10-13T12:00", nil, false, 3 * time.Hour},
		{"daily full", "daily", "00:00", "24:00", "2026-10-13T23:59", nil, true, 24 * time.Hour},
		{"weekly Monday wrap", "weekly", "23:00", "02:00", "2026-10-13T01:30", []int{1}, true, 3 * time.Hour},
		{"weekly wrong start day", "weekly", "23:00", "02:00", "2026-10-13T01:30", []int{2}, false, 3 * time.Hour},
		{"weekly Sunday wrap", "weekly", "23:00", "02:00", "2026-10-12T01:30", []int{0}, true, 3 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.ParseInLocation("2006-01-02T15:04", tc.now, groupOpeningLocation)
			require.NoError(t, err)
			start, end, active := timedRatioWindow(GroupTimedRatioRule{Kind: tc.kind, Start: tc.start, End: tc.end, Weekdays: tc.days}, now.UTC())
			require.Equal(t, tc.active, active)
			require.Equal(t, tc.duration, end.Sub(start))
			if active {
				require.False(t, now.Before(start))
				require.True(t, now.Before(end))
			}
		})
	}
}

func TestTimedRatioValidationAndResolution(t *testing.T) {
	c := groupRatioSetting.GroupTimedRatios
	original, err := common.Marshal(c)
	require.NoError(t, err)
	oldRatios, oldSpecial := GroupRatio2JSONString(), GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, common.Unmarshal(original, c))
		require.NoError(t, UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, UpdateGroupGroupRatioByJSONString(oldSpecial))
	})
	require.NoError(t, UpdateGroupRatioByJSONString(`{"a":2}`))
	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"vip":{"a":1.5}}`))
	valid := `{"a":[{"enabled":false,"ratio":9,"kind":"daily","start":"00:00","end":"24:00"},{"enabled":true,"ratio":0,"kind":"daily","start":"09:00","end":"10:00","remark":"free hour"},{"enabled":true,"ratio":0.5,"kind":"daily","start":"09:00","end":"11:00"}]}`
	require.NoError(t, common.Unmarshal([]byte(valid), c))
	for _, invalid := range []string{
		`null`, `[]`, `{" ":[]}`,
		`{"a":[{"ratio":-1,"kind":"daily","start":"09:00","end":"10:00"}]}`,
		`{"a":[{"kind":"daily","start":"09:00","end":"10:00"}]}`,
		`{"a":[{"ratio":1,"kind":"hourly","start":"02:00","end":"03:00"}]}`,
		`{"a":[{"ratio":1,"kind":"daily","start":"10:00","end":"10:00"}]}`,
		`{"a":[{"ratio":1,"kind":"weekly","start":"09:00","end":"10:00","weekdays":[]}]}`,
		`{"a":[{"ratio":1,"kind":"weekly","start":"09:00","end":"10:00","weekdays":[7]}]}`,
		`{"a":[{"ratio":1,"kind":"once","start":"2026-02-30T09:00","end":"2026-03-01T10:00"}]}`,
		`{"a":[{"ratio":1,"kind":"once","start":"2026-10-13T10:00","end":"2026-10-13T09:00"}]}`,
		`{"a":[{"ratio":1,"kind":"unknown","start":"09:00","end":"10:00"}]}`,
	} {
		require.Error(t, common.Unmarshal([]byte(invalid), c), invalid)
	}
	now := time.Date(2026, 10, 13, 9, 30, 0, 0, groupOpeningLocation)
	info := ResolveGroupRatioAt("vip", "a", now)
	require.Zero(t, info.GroupRatio)
	require.False(t, info.HasSpecialRatio)
	require.Equal(t, -1.0, info.GroupSpecialRatio)
	require.Equal(t, 1.5, info.TimedRatio.OriginalRatio)
	require.Equal(t, "free hour", info.TimedRatio.Remark)
	require.Equal(t, 0.5, ResolveGroupRatioAt("vip", "a", now.Add(30*time.Minute)).GroupRatio)
	expired := ResolveGroupRatioAt("vip", "a", now.Add(2*time.Hour))
	require.Equal(t, 1.5, expired.GroupRatio)
	require.True(t, expired.HasSpecialRatio)
	require.Nil(t, expired.TimedRatio)
	require.Equal(t, 2.0, ResolveGroupRatioAt("", "a", now.Add(2*time.Hour)).GroupRatio)
	require.NoError(t, common.Unmarshal([]byte(`{}`), c))
	require.Equal(t, "free hour", info.TimedRatio.Remark)
	require.Equal(t, int64(3600), info.TimedRatio.End-info.TimedRatio.Start)
}
