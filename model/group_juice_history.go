package model

import (
	"math/big"
	"sort"
	"strings"
	"time"
)

const (
	GroupJuiceHistorySourceManual    = "manual"
	GroupJuiceHistorySourceScheduled = "scheduled"
)

type GroupJuiceHistory struct {
	Id        int    `json:"id"`
	Group     string `json:"group" gorm:"size:191;index:idx_group_juice_history_group_time,priority:1"`
	OldJuice  string `json:"old_juice" gorm:"type:varchar(255)"`
	NewJuice  string `json:"new_juice" gorm:"type:varchar(255)"`
	ChannelId int    `json:"channel_id" gorm:"index"`
	Source    string `json:"source" gorm:"size:32;index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index:idx_group_juice_history_group_time,priority:2;index"`
}

type GroupJuiceHistoryPoint struct {
	Ts        int64  `json:"ts"`
	Juice     string `json:"juice"`
	ChannelId int    `json:"channel_id,omitempty"`
	Source    string `json:"source,omitempty"`
}

type GroupJuiceHistorySeries struct {
	Group  string                   `json:"group"`
	Points []GroupJuiceHistoryPoint `json:"points"`
}

type GroupJuiceHistorySummary struct {
	StartTs int64                     `json:"start_ts"`
	EndTs   int64                     `json:"end_ts"`
	Groups  []GroupJuiceHistorySeries `json:"groups"`
}

func RecordGroupJuiceChanges(previous, current map[string]GroupJuiceStats, channelId int, source string) error {
	if source == "" {
		source = GroupJuiceHistorySourceManual
	}

	groups := make([]string, 0, len(current))
	for group := range current {
		if _, ok := previous[group]; ok {
			groups = append(groups, group)
		}
	}
	sort.Strings(groups)

	now := time.Now().Unix()
	records := make([]GroupJuiceHistory, 0, len(groups))
	for _, group := range groups {
		oldJuice := strings.TrimSpace(previous[group].Juice)
		newJuice := strings.TrimSpace(current[group].Juice)
		if equalJuiceValues(oldJuice, newJuice) {
			continue
		}
		records = append(records, GroupJuiceHistory{
			Group:     normalizePerfMetricGroupName(group),
			OldJuice:  oldJuice,
			NewJuice:  newJuice,
			ChannelId: channelId,
			Source:    source,
			CreatedAt: now,
		})
	}
	if len(records) == 0 {
		return nil
	}
	return DB.Create(&records).Error
}

func selectGroupJuiceStats(stats map[string]GroupJuiceStats, groups []string) map[string]GroupJuiceStats {
	if len(groups) == 0 {
		groups = []string{"default"}
	}
	selected := make(map[string]GroupJuiceStats, len(groups))
	for _, group := range groups {
		group = normalizePerfMetricGroupName(group)
		if value, ok := stats[group]; ok {
			selected[group] = value
		}
	}
	return selected
}

func GetGroupJuiceHistorySummary(startTs, endTs int64) (GroupJuiceHistorySummary, error) {
	now := time.Now().Unix()
	if endTs <= 0 {
		endTs = now
	}
	if startTs <= 0 || startTs >= endTs {
		startTs = endTs - 7*24*60*60
	}

	currentStats, err := GetGroupJuiceStats()
	if err != nil {
		return GroupJuiceHistorySummary{}, err
	}
	groupSet := make(map[string]struct{}, len(currentStats))
	startJuices := make(map[string]string, len(currentStats))
	for group, stats := range currentStats {
		group = normalizePerfMetricGroupName(group)
		groupSet[group] = struct{}{}
		startJuices[group] = stats.Juice
	}

	var allRows []GroupJuiceHistory
	if err := DB.Where("created_at >= ?", startTs).
		Order("created_at ASC, id ASC").
		Find(&allRows).Error; err != nil {
		return GroupJuiceHistorySummary{}, err
	}
	for _, row := range allRows {
		groupSet[row.Group] = struct{}{}
	}
	for i := len(allRows) - 1; i >= 0; i-- {
		row := allRows[i]
		if row.CreatedAt <= startTs {
			continue
		}
		startJuices[row.Group] = row.OldJuice
	}

	groupNames := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groupNames = append(groupNames, group)
	}
	sort.Strings(groupNames)

	seriesMap := make(map[string][]GroupJuiceHistoryPoint, len(groupNames))
	for _, group := range groupNames {
		juice, ok := startJuices[group]
		if !ok {
			continue
		}
		seriesMap[group] = []GroupJuiceHistoryPoint{{Ts: startTs, Juice: juice}}
	}
	for _, row := range allRows {
		if row.CreatedAt < startTs || row.CreatedAt > endTs {
			continue
		}
		seriesMap[row.Group] = append(seriesMap[row.Group], GroupJuiceHistoryPoint{
			Ts:        row.CreatedAt,
			Juice:     row.NewJuice,
			ChannelId: row.ChannelId,
			Source:    row.Source,
		})
	}

	groups := make([]GroupJuiceHistorySeries, 0, len(groupNames))
	for _, group := range groupNames {
		points := seriesMap[group]
		if len(points) == 0 {
			continue
		}
		groups = append(groups, GroupJuiceHistorySeries{Group: group, Points: points})
	}

	return GroupJuiceHistorySummary{StartTs: startTs, EndTs: endTs, Groups: groups}, nil
}

func equalJuiceValues(left, right string) bool {
	leftValue, leftOK := new(big.Rat).SetString(left)
	rightValue, rightOK := new(big.Rat).SetString(right)
	return leftOK && rightOK && leftValue.Cmp(rightValue) == 0
}
