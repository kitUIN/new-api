package ratio_setting

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGroupOpeningHours(t *testing.T) {
	original, err := common.Marshal(groupRatioSetting.GroupOpeningHours)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, common.Unmarshal(original, groupRatioSetting.GroupOpeningHours)) })
	require.NoError(t, common.Unmarshal([]byte(`{"day":[{"start":"09:00","end":"12:00"},{"start":"14:00","end":"18:00"}],"night":[{"start":"22:00","end":"02:00"}],"full":[{"start":"00:00","end":"24:00"}],"empty":[]}`), groupRatioSetting.GroupOpeningHours))
	for _, tc := range []struct {
		group, clock string
		open         bool
	}{
		{"day", "08:59", false}, {"day", "09:00", true}, {"day", "12:00", false},
		{"day", "14:00", true}, {"day", "18:00", false}, {"night", "22:00", true},
		{"night", "00:00", true}, {"night", "01:59", true}, {"night", "02:00", false},
		{"night", "21:59", false}, {"full", "23:59", true}, {"full", "00:00", true},
		{"empty", "12:00", true}, {"unknown", "12:00", true},
	} {
		t.Run(tc.group+tc.clock, func(t *testing.T) {
			now, err := time.ParseInLocation("2006-01-02 15:04", "2026-09-23 "+tc.clock, groupOpeningLocation)
			require.NoError(t, err)
			err = CheckGroupOpenAt(tc.group, now.UTC())
			if tc.open {
				require.NoError(t, err)
			} else {
				var closed *GroupNotOpenError
				require.ErrorAs(t, err, &closed)
				require.Contains(t, err.Error(), "未开放")
				require.Contains(t, err.Error(), "UTC+8")
			}
		})
	}
}

func TestGroupOpeningHoursRejectsInvalidWithoutReplacing(t *testing.T) {
	config := &GroupOpeningHoursConfig{}
	valid := []byte(`{"test":[{"start":"09:00","end":"18:00"}]}`)
	require.NoError(t, common.Unmarshal(valid, config))
	for _, invalid := range []string{
		`null`, `[]`, `{"":[]}`, `{"test":[{}]}`,
		`{"test":[{"start":"9:00","end":"18:00"}]}`,
		`{"test":[{"start":"24:00","end":"18:00"}]}`,
		`{"test":[{"start":"09:00","end":"25:00"}]}`,
		`{"test":[{"start":"09:00","end":"09:00"}]}`,
	} {
		require.Error(t, common.Unmarshal([]byte(invalid), config))
		current, err := common.Marshal(config)
		require.NoError(t, err)
		require.JSONEq(t, string(valid), string(current))
	}
}
