package ratio_setting

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Opening intervals repeat daily in UTC+8, including start and excluding end.
// An empty list means unrestricted. End before start means the following day.
type GroupOpeningInterval struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type GroupOpeningHoursConfig struct {
	mu     sync.RWMutex
	groups map[string][]GroupOpeningInterval
}

var groupOpeningLocation = time.FixedZone("UTC+8", 8*60*60)

func openingMinute(value string, end bool) (int, error) {
	if end && value == "24:00" {
		return 1440, nil
	}
	t, err := time.Parse("15:04", value)
	if err != nil || t.Format("15:04") != value {
		return 0, fmt.Errorf("开放时间必须为 HH:mm 格式: %s", value)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func (c *GroupOpeningHoursConfig) UnmarshalJSON(data []byte) error {
	var groups map[string][]GroupOpeningInterval
	if err := common.Unmarshal(data, &groups); err != nil {
		return err
	}
	if groups == nil {
		return fmt.Errorf("开放时间必须为 JSON 对象")
	}
	for group, intervals := range groups {
		if strings.TrimSpace(group) == "" {
			return fmt.Errorf("开放时间的分组名不能为空")
		}
		for _, interval := range intervals {
			start, err := openingMinute(interval.Start, false)
			if err != nil {
				return err
			}
			end, err := openingMinute(interval.End, true)
			if err != nil {
				return err
			}
			if start == end {
				return fmt.Errorf("分组 %s 的开放时间起止不能相同", group)
			}
		}
	}
	c.mu.Lock()
	c.groups = groups
	c.mu.Unlock()
	return nil
}

func (c *GroupOpeningHoursConfig) MarshalJSON() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.groups == nil {
		return []byte("{}"), nil
	}
	return common.Marshal(c.groups)
}

func CheckGroupOpeningHours(value string) error {
	return common.Unmarshal([]byte(value), &GroupOpeningHoursConfig{})
}

type GroupNotOpenError struct{ Group, Hours string }

func (e *GroupNotOpenError) Error() string {
	return fmt.Sprintf("分组 %s 未开放，开放时间：每天 %s（北京时间 UTC+8）", e.Group, e.Hours)
}

func CheckGroupOpenAt(group string, now time.Time) error {
	c := groupRatioSetting.GroupOpeningHours
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	intervals := c.groups[group]
	if len(intervals) == 0 {
		return nil
	}
	local := now.In(groupOpeningLocation)
	minute := local.Hour()*60 + local.Minute()
	descriptions := make([]string, 0, len(intervals))
	for _, interval := range intervals {
		start, _ := openingMinute(interval.Start, false)
		end, _ := openingMinute(interval.End, true)
		if (start < end && minute >= start && minute < end) || (start > end && (minute >= start || minute < end)) {
			return nil
		}
		description := interval.Start + "–" + interval.End
		if start > end {
			description += "（次日）"
		}
		descriptions = append(descriptions, description)
	}
	return &GroupNotOpenError{Group: group, Hours: strings.Join(descriptions, "、")}
}
