package model

import (
	"math"
	"sort"
	"strings"
	"time"
)

const defaultPerfMetricsLimit = 200

type PerfModelSummary struct {
	ModelName    string  `json:"model_name"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
	RequestCount int64   `json:"request_count,omitempty"`
}

type PerfMetricsSummary struct {
	Models []PerfModelSummary `json:"models"`
}

type PerformanceSeriesPoint struct {
	Ts           int64   `json:"ts"`
	AvgTTFTMs    float64 `json:"avg_ttft_ms"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

type PerformanceGroup struct {
	Group        string                   `json:"group"`
	AvgTTFTMs    float64                  `json:"avg_ttft_ms"`
	AvgLatencyMs float64                  `json:"avg_latency_ms"`
	SuccessRate  float64                  `json:"success_rate"`
	AvgTps       float64                  `json:"avg_tps"`
	Series       []PerformanceSeriesPoint `json:"series"`
}

type PerformanceMetrics struct {
	ModelName    string             `json:"model_name"`
	SeriesSchema string             `json:"series_schema,omitempty"`
	Groups       []PerformanceGroup `json:"groups"`
}

type perfModelSummaryRow struct {
	ModelName               string
	RequestCount            int64
	SuccessCount            int64
	SuccessUseTime          int64
	TpsUseTime              int64
	TpsCompletionTokens     int64
	SuccessCompletionTokens int64
}

type perfAccumulator struct {
	requestCount     int64
	successCount     int64
	successUseTime   int64
	tpsUseTime       int64
	completionTokens int64
}

func (a perfAccumulator) summary() (avgLatencyMs float64, successRate float64, avgTps float64) {
	if a.requestCount > 0 {
		successRate = roundFloat(float64(a.successCount)/float64(a.requestCount)*100, 2)
	}
	if a.successCount > 0 {
		avgLatencyMs = roundFloat(float64(a.successUseTime)/float64(a.successCount)*1000, 2)
	}
	if a.tpsUseTime > 0 {
		avgTps = roundFloat(float64(a.completionTokens)/float64(a.tpsUseTime), 2)
	}
	return avgLatencyMs, successRate, avgTps
}

func GetPerfMetricsSummary(hours int, limit int) (PerfMetricsSummary, error) {
	startTimestamp := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	if limit <= 0 {
		limit = defaultPerfMetricsLimit
	}

	var rows []perfModelSummaryRow
	err := LOG_DB.Table("logs").
		Select(
			`model_name,
			COUNT(*) AS request_count,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_count,
			SUM(CASE WHEN type = ? THEN use_time ELSE 0 END) AS success_use_time,
			SUM(CASE WHEN type = ? AND use_time > 0 THEN use_time ELSE 0 END) AS tps_use_time,
			SUM(CASE WHEN type = ? AND use_time > 0 THEN completion_tokens ELSE 0 END) AS tps_completion_tokens,
			SUM(CASE WHEN type = ? THEN completion_tokens ELSE 0 END) AS success_completion_tokens`,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume,
		).
		Where("created_at >= ?", startTimestamp).
		Where("model_name <> ''").
		Where("type IN ?", []int{LogTypeConsume, LogTypeError}).
		Group("model_name").
		Order("request_count DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return PerfMetricsSummary{}, err
	}

	models := make([]PerfModelSummary, 0, len(rows))
	for _, row := range rows {
		acc := perfAccumulator{
			requestCount:     row.RequestCount,
			successCount:     row.SuccessCount,
			successUseTime:   row.SuccessUseTime,
			tpsUseTime:       row.TpsUseTime,
			completionTokens: row.TpsCompletionTokens,
		}
		avgLatencyMs, successRate, avgTps := acc.summary()
		models = append(models, PerfModelSummary{
			ModelName:    row.ModelName,
			AvgLatencyMs: avgLatencyMs,
			SuccessRate:  successRate,
			AvgTps:       avgTps,
			RequestCount: row.RequestCount,
		})
	}

	return PerfMetricsSummary{Models: models}, nil
}

func GetPerformanceMetrics(modelName string, hours int) (PerformanceMetrics, error) {
	modelName = strings.TrimSpace(modelName)
	startTimestamp := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()

	var logs []Log
	err := LOG_DB.Model(&Log{}).
		Where("created_at >= ?", startTimestamp).
		Where("model_name = ?", modelName).
		Where("type IN ?", []int{LogTypeConsume, LogTypeError}).
		Order("created_at ASC").
		Find(&logs).Error
	if err != nil {
		return PerformanceMetrics{}, err
	}

	type groupedAccumulator struct {
		total  perfAccumulator
		series map[int64]*perfAccumulator
	}

	groups := make(map[string]*groupedAccumulator)
	for _, log := range logs {
		groupName := strings.TrimSpace(log.Group)
		if groupName == "" {
			groupName = "default"
		}
		group, ok := groups[groupName]
		if !ok {
			group = &groupedAccumulator{series: make(map[int64]*perfAccumulator)}
			groups[groupName] = group
		}

		applyPerfLog(&group.total, log)
		bucketTs := log.CreatedAt - log.CreatedAt%3600
		bucket := group.series[bucketTs]
		if bucket == nil {
			bucket = &perfAccumulator{}
			group.series[bucketTs] = bucket
		}
		applyPerfLog(bucket, log)
	}

	groupNames := make([]string, 0, len(groups))
	for groupName := range groups {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)

	resultGroups := make([]PerformanceGroup, 0, len(groupNames))
	for _, groupName := range groupNames {
		group := groups[groupName]
		avgLatencyMs, successRate, avgTps := group.total.summary()

		bucketTsList := make([]int64, 0, len(group.series))
		for bucketTs := range group.series {
			bucketTsList = append(bucketTsList, bucketTs)
		}
		sort.Slice(bucketTsList, func(i, j int) bool {
			return bucketTsList[i] < bucketTsList[j]
		})

		series := make([]PerformanceSeriesPoint, 0, len(bucketTsList))
		for _, bucketTs := range bucketTsList {
			bucketLatencyMs, bucketSuccessRate, bucketTps := group.series[bucketTs].summary()
			series = append(series, PerformanceSeriesPoint{
				Ts:           bucketTs,
				AvgTTFTMs:    bucketLatencyMs,
				AvgLatencyMs: bucketLatencyMs,
				SuccessRate:  bucketSuccessRate,
				AvgTps:       bucketTps,
			})
		}

		resultGroups = append(resultGroups, PerformanceGroup{
			Group:        groupName,
			AvgTTFTMs:    avgLatencyMs,
			AvgLatencyMs: avgLatencyMs,
			SuccessRate:  successRate,
			AvgTps:       avgTps,
			Series:       series,
		})
	}

	return PerformanceMetrics{
		ModelName:    modelName,
		SeriesSchema: "hourly; avg_ttft_ms currently mirrors avg_latency_ms because logs do not store TTFT separately",
		Groups:       resultGroups,
	}, nil
}

func applyPerfLog(acc *perfAccumulator, log Log) {
	acc.requestCount++
	if log.Type != LogTypeConsume {
		return
	}
	acc.successCount++
	acc.successUseTime += int64(log.UseTime)
	if log.UseTime > 0 {
		acc.tpsUseTime += int64(log.UseTime)
		acc.completionTokens += int64(log.CompletionTokens)
	}
}

func roundFloat(value float64, precision int) float64 {
	if precision < 0 {
		return value
	}
	multiplier := math.Pow10(precision)
	return math.Round(value*multiplier) / multiplier
}
