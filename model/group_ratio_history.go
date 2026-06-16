package model

import (
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	GroupRatioHistorySourceManual = "manual"
	GroupRatioHistorySourceSync   = "sync"
)

type GroupRatioHistory struct {
	Id        int     `json:"id"`
	Group     string  `json:"group" gorm:"size:191;index:idx_group_ratio_history_group_time,priority:1"`
	OldRatio  float64 `json:"old_ratio"`
	NewRatio  float64 `json:"new_ratio"`
	Source    string  `json:"source" gorm:"size:32;index"`
	CreatedAt int64   `json:"created_at" gorm:"bigint;index:idx_group_ratio_history_group_time,priority:2;index"`
}

type GroupRatioHistoryPoint struct {
	Ts     int64   `json:"ts"`
	Ratio  float64 `json:"ratio"`
	Source string  `json:"source,omitempty"`
}

type GroupRatioHistorySeries struct {
	Group  string                   `json:"group"`
	Points []GroupRatioHistoryPoint `json:"points"`
}

type GroupRatioHistorySummary struct {
	StartTs int64                     `json:"start_ts"`
	EndTs   int64                     `json:"end_ts"`
	Groups  []GroupRatioHistorySeries `json:"groups"`
}

func RecordGroupRatioChanges(previous, current map[string]float64, source string) error {
	changes := ratio_setting.CompareGroupRatioChanges(previous, current)
	if len(changes) == 0 {
		return nil
	}
	if source == "" {
		source = GroupRatioHistorySourceManual
	}

	now := time.Now().Unix()
	records := make([]GroupRatioHistory, 0, len(changes))
	for _, change := range changes {
		if change.Type == ratio_setting.GroupRatioChangeAdded {
			continue
		}
		group := strings.TrimSpace(change.Group)
		if group == "" {
			group = "default"
		}
		records = append(records, GroupRatioHistory{
			Group:     group,
			OldRatio:  change.OldRatio,
			NewRatio:  change.NewRatio,
			Source:    source,
			CreatedAt: now,
		})
	}
	if len(records) == 0 {
		return nil
	}
	return DB.Create(&records).Error
}

func GetGroupRatioHistorySummary(startTs, endTs int64) (GroupRatioHistorySummary, error) {
	now := time.Now().Unix()
	if endTs <= 0 {
		endTs = now
	}
	if startTs <= 0 || startTs >= endTs {
		startTs = endTs - 7*24*60*60
	}

	currentRatios := ratio_setting.GetGroupRatioCopy()
	groupSet := make(map[string]struct{}, len(currentRatios))
	for group := range currentRatios {
		group = strings.TrimSpace(group)
		if group == "" {
			group = "default"
		}
		groupSet[group] = struct{}{}
	}

	var allRows []GroupRatioHistory
	if err := DB.Where("created_at >= ?", startTs).
		Order("created_at ASC, id ASC").
		Find(&allRows).Error; err != nil {
		return GroupRatioHistorySummary{}, err
	}
	for _, row := range allRows {
		groupSet[row.Group] = struct{}{}
	}

	startRatios := make(map[string]float64, len(currentRatios))
	for group, ratio := range currentRatios {
		group = strings.TrimSpace(group)
		if group == "" {
			group = "default"
		}
		startRatios[group] = ratio
	}
	for i := len(allRows) - 1; i >= 0; i-- {
		row := allRows[i]
		if row.CreatedAt <= startTs {
			continue
		}
		startRatios[row.Group] = row.OldRatio
	}

	groupNames := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groupNames = append(groupNames, group)
	}
	sort.Strings(groupNames)

	seriesMap := make(map[string][]GroupRatioHistoryPoint, len(groupNames))
	for _, group := range groupNames {
		ratio, ok := startRatios[group]
		if !ok {
			ratio = 1
		}
		seriesMap[group] = []GroupRatioHistoryPoint{
			{Ts: startTs, Ratio: ratio},
		}
	}

	for _, row := range allRows {
		if row.CreatedAt < startTs || row.CreatedAt > endTs {
			continue
		}
		seriesMap[row.Group] = append(seriesMap[row.Group], GroupRatioHistoryPoint{
			Ts:     row.CreatedAt,
			Ratio:  row.NewRatio,
			Source: row.Source,
		})
	}

	groups := make([]GroupRatioHistorySeries, 0, len(groupNames))
	for _, group := range groupNames {
		groups = append(groups, GroupRatioHistorySeries{
			Group:  group,
			Points: seriesMap[group],
		})
	}

	return GroupRatioHistorySummary{
		StartTs: startTs,
		EndTs:   endTs,
		Groups:  groups,
	}, nil
}
