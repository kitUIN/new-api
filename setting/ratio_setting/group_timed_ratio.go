package ratio_setting

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

// Rules use Beijing time. Intervals include start and exclude end; the first
// matching rule wins. Weekly days identify the day on which an interval starts.
type GroupTimedRatioRule struct {
	Enabled  bool     `json:"enabled"`
	Ratio    *float64 `json:"ratio"`
	Kind     string   `json:"kind"`
	Start    string   `json:"start"`
	End      string   `json:"end"`
	Weekdays []int    `json:"weekdays,omitempty"`
	Remark   string   `json:"remark"`
}

type GroupTimedRatioConfig struct {
	mu     sync.RWMutex
	groups map[string][]GroupTimedRatioRule
}

func timedRatioBoundary(rule GroupTimedRatioRule, value string, end bool) (time.Time, error) {
	if rule.Kind == "once" {
		parsed, err := time.ParseInLocation("2006-01-02T15:04", value, groupOpeningLocation)
		if err != nil || parsed.Format("2006-01-02T15:04") != value {
			return time.Time{}, fmt.Errorf("一次性限时倍率时间必须为 YYYY-MM-DDTHH:mm: %s", value)
		}
		return parsed, nil
	}
	minute, err := openingMinute(value, end)
	if err != nil {
		return time.Time{}, err
	}
	if rule.Kind == "hourly" && (minute > 60 || (!end && minute == 60)) {
		return time.Time{}, fmt.Errorf("每小时限时倍率使用 00:mm，结束允许 01:00")
	}
	return time.Date(2000, 1, 1, 0, minute, 0, 0, groupOpeningLocation), nil
}

func (c *GroupTimedRatioConfig) UnmarshalJSON(data []byte) error {
	var groups map[string][]GroupTimedRatioRule
	if err := common.Unmarshal(data, &groups); err != nil {
		return err
	}
	if groups == nil {
		return fmt.Errorf("限时倍率必须为 JSON 对象")
	}
	for group, rules := range groups {
		if strings.TrimSpace(group) == "" {
			return fmt.Errorf("限时倍率分组名不能为空")
		}
		for _, rule := range rules {
			if rule.Ratio == nil || math.IsNaN(*rule.Ratio) || math.IsInf(*rule.Ratio, 0) || *rule.Ratio < 0 {
				return fmt.Errorf("分组 %s 的限时倍率必须为非负数", group)
			}
			switch rule.Kind {
			case "once", "hourly", "daily", "weekly":
			default:
				return fmt.Errorf("不支持的限时倍率周期: %s", rule.Kind)
			}
			start, err := timedRatioBoundary(rule, rule.Start, false)
			if err != nil {
				return err
			}
			end, err := timedRatioBoundary(rule, rule.End, true)
			if err != nil {
				return err
			}
			if start.Equal(end) || (rule.Kind == "once" && !end.After(start)) {
				return fmt.Errorf("分组 %s 的限时倍率起止时间无效", group)
			}
			if rule.Kind == "weekly" && len(rule.Weekdays) == 0 {
				return fmt.Errorf("每周限时倍率必须选择星期")
			}
			for _, day := range rule.Weekdays {
				if day < 0 || day > 6 {
					return fmt.Errorf("星期必须在 0 至 6 之间")
				}
			}
		}
	}
	c.mu.Lock()
	c.groups = groups
	c.mu.Unlock()
	return nil
}

func (c *GroupTimedRatioConfig) MarshalJSON() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.groups == nil {
		return []byte("{}"), nil
	}
	return common.Marshal(c.groups)
}

func CheckGroupTimedRatios(value string) error {
	return common.Unmarshal([]byte(value), &GroupTimedRatioConfig{})
}

func timedRatioWindow(rule GroupTimedRatioRule, now time.Time) (time.Time, time.Time, bool) {
	local := now.In(groupOpeningLocation)
	if rule.Kind == "once" {
		start, _ := timedRatioBoundary(rule, rule.Start, false)
		end, _ := timedRatioBoundary(rule, rule.End, true)
		return start, end, !local.Before(start) && local.Before(end)
	}
	startMinute, _ := openingMinute(rule.Start, false)
	endMinute, _ := openingMinute(rule.End, true)
	period := 24 * time.Hour
	anchor := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, groupOpeningLocation)
	if rule.Kind == "hourly" {
		period = time.Hour
		anchor = anchor.Add(time.Duration(local.Hour()) * time.Hour)
	}
	start := anchor.Add(time.Duration(startMinute) * time.Minute)
	end := anchor.Add(time.Duration(endMinute) * time.Minute)
	if !end.After(start) {
		end = end.Add(period)
	}
	if local.Before(start) {
		start = start.Add(-period)
		end = end.Add(-period)
	}
	if rule.Kind == "weekly" {
		matched := false
		for _, day := range rule.Weekdays {
			if int(start.Weekday()) == day {
				matched = true
				break
			}
		}
		if !matched {
			return start, end, false
		}
	}
	return start, end, !local.Before(start) && local.Before(end)
}

// ResolveGroupRatioAt is shared by routing, displayed effective prices and billing.
// A timed rule overrides both the base ratio and any user-group special ratio.
func ResolveGroupRatioAt(userGroup, group string, now time.Time) types.GroupRatioInfo {
	info := types.GroupRatioInfo{GroupRatio: GetGroupRatio(group), GroupSpecialRatio: -1}
	if special, ok := GetGroupGroupRatio(userGroup, group); ok {
		info.GroupRatio, info.GroupSpecialRatio, info.HasSpecialRatio = special, special, true
	}
	c := groupRatioSetting.GroupTimedRatios
	if c == nil {
		return info
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, rule := range c.groups[group] {
		if !rule.Enabled {
			continue
		}
		start, end, active := timedRatioWindow(rule, now)
		if !active {
			continue
		}
		info.TimedRatio = &types.GroupTimedRatioSnapshot{
			OriginalRatio: info.GroupRatio, Ratio: *rule.Ratio,
			Start: start.Unix(), End: end.Unix(), Remark: rule.Remark,
		}
		info.GroupRatio, info.GroupSpecialRatio, info.HasSpecialRatio = *rule.Ratio, -1, false
		break
	}
	return info
}
