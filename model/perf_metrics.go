package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const defaultPerfMetricsLimit = 200

type PerfMetricBucket struct {
	Id                int    `json:"id"`
	BucketStart       int64  `json:"bucket_start" gorm:"bigint;uniqueIndex:idx_perf_metric_bucket"`
	BucketSeconds     int64  `json:"bucket_seconds" gorm:"bigint;uniqueIndex:idx_perf_metric_bucket"`
	ModelName         string `json:"model_name" gorm:"size:191;uniqueIndex:idx_perf_metric_bucket;index"`
	Group             string `json:"group" gorm:"size:191;uniqueIndex:idx_perf_metric_bucket;index"`
	RequestCount      int64  `json:"request_count" gorm:"bigint;default:0"`
	SuccessCount      int64  `json:"success_count" gorm:"bigint;default:0"`
	TotalLatencyMs    int64  `json:"total_latency_ms" gorm:"bigint;default:0"`
	TotalTTFTMs       int64  `json:"total_ttft_ms" gorm:"bigint;default:0"`
	TTFTCount         int64  `json:"ttft_count" gorm:"bigint;default:0"`
	CompletionTokens  int64  `json:"completion_tokens" gorm:"bigint;default:0"`
	TotalTPSLatencyMs int64  `json:"total_tps_latency_ms" gorm:"bigint;default:0"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt         int64  `json:"updated_at" gorm:"bigint"`
}

type PerfMetricSample struct {
	Timestamp        int64
	ModelName        string
	Group            string
	Success          bool
	LatencyMs        int64
	TTFTMs           int64
	CompletionTokens int64
	TPSLatencyMs     int64
}

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

type PerfGroupHealthBucket struct {
	Ts           int64   `json:"ts"`
	EndTs        int64   `json:"end_ts"`
	RequestCount int64   `json:"request_count"`
	SuccessCount int64   `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTTFTMs    float64 `json:"avg_ttft_ms"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	AvgTps       float64 `json:"avg_tps"`
	Status       string  `json:"status"`
}

type PerfGroupHealth struct {
	Group         string                  `json:"group"`
	Ratio         float64                 `json:"ratio"`
	ProviderCount int                     `json:"provider_count"`
	RequestCount  int64                   `json:"request_count"`
	SuccessRate   float64                 `json:"success_rate"`
	AvgTTFTMs     float64                 `json:"avg_ttft_ms"`
	AvgLatencyMs  float64                 `json:"avg_latency_ms"`
	AvgTps        float64                 `json:"avg_tps"`
	Buckets       []PerfGroupHealthBucket `json:"buckets"`
}

type PerfGroupHealthSummary struct {
	WindowHours     int               `json:"window_hours"`
	IntervalMinutes int               `json:"interval_minutes"`
	BucketCount     int               `json:"bucket_count"`
	SeriesSchema    string            `json:"series_schema,omitempty"`
	Groups          []PerfGroupHealth `json:"groups"`
}

type RankingGroupPerfStat struct {
	Group        string  `json:"group"`
	RequestCount int64   `json:"request_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTTFTMs    float64 `json:"avg_ttft_ms"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	AvgTps       float64 `json:"avg_tps"`
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

type perfMetricKey struct {
	bucketStart   int64
	bucketSeconds int64
	modelName     string
	group         string
}

type perfAccumulator struct {
	requestCount      int64
	successCount      int64
	totalLatencyMs    int64
	totalTTFTMs       int64
	ttftCount         int64
	completionTokens  int64
	totalTPSLatencyMs int64
}

var (
	perfMetricsMu      sync.Mutex
	pendingPerfMetrics = make(map[perfMetricKey]perfAccumulator)
)

func RecordPerfMetricSample(sample PerfMetricSample) {
	config := common.GetPerfMetricsConfig()
	if !config.Enabled {
		return
	}
	modelName := strings.TrimSpace(sample.ModelName)
	if modelName == "" {
		return
	}
	group := strings.TrimSpace(sample.Group)
	if group == "" {
		group = "default"
	}
	if sample.Timestamp <= 0 {
		sample.Timestamp = time.Now().Unix()
	}
	if sample.LatencyMs < 0 {
		sample.LatencyMs = 0
	}
	if sample.TTFTMs < 0 {
		sample.TTFTMs = 0
	}
	if sample.CompletionTokens < 0 {
		sample.CompletionTokens = 0
	}
	if sample.TPSLatencyMs < 0 {
		sample.TPSLatencyMs = 0
	}

	bucketSeconds := config.BucketSeconds
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	key := perfMetricKey{
		bucketStart:   sample.Timestamp - sample.Timestamp%bucketSeconds,
		bucketSeconds: bucketSeconds,
		modelName:     modelName,
		group:         group,
	}

	perfMetricsMu.Lock()
	acc := pendingPerfMetrics[key]
	acc.requestCount++
	if sample.Success {
		acc.successCount++
		acc.totalLatencyMs += sample.LatencyMs
		if sample.TTFTMs > 0 {
			acc.totalTTFTMs += sample.TTFTMs
			acc.ttftCount++
		}
		if sample.CompletionTokens > 0 && sample.TPSLatencyMs > 0 {
			acc.completionTokens += sample.CompletionTokens
			acc.totalTPSLatencyMs += sample.TPSLatencyMs
		}
	}
	pendingPerfMetrics[key] = acc
	perfMetricsMu.Unlock()
}

func FlushPerfMetrics() error {
	snapshot := takePendingPerfMetrics()
	if len(snapshot) == 0 {
		return nil
	}
	if LOG_DB == nil {
		restorePendingPerfMetrics(snapshot)
		return errors.New("log database is not initialized")
	}

	now := time.Now().Unix()
	err := LOG_DB.Transaction(func(tx *gorm.DB) error {
		for key, acc := range snapshot {
			if acc.requestCount <= 0 {
				continue
			}
			if err := upsertPerfMetricBucket(tx, key, acc, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		restorePendingPerfMetrics(snapshot)
		return err
	}

	config := common.GetPerfMetricsConfig()
	if config.RetentionDays > 0 {
		return CleanupPerfMetrics(config.RetentionDays)
	}
	return nil
}

func StartPerfMetricsFlushLoop() {
	startPerfMetricsFlushLoop(context.Background(), sleepPerfMetricsFlushInterval)
}

func startPerfMetricsFlushLoop(ctx context.Context, sleep func(context.Context, time.Duration) bool) {
	for {
		if !sleep(ctx, perfMetricsFlushInterval()) {
			return
		}
		if err := FlushPerfMetrics(); err != nil {
			common.SysError("failed to flush performance metrics: " + err.Error())
		}
	}
}

func perfMetricsFlushInterval() time.Duration {
	config := common.GetPerfMetricsConfig()
	interval := config.FlushInterval
	if interval <= 0 {
		interval = 5
	}
	return time.Duration(interval) * time.Minute
}

func sleepPerfMetricsFlushInterval(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func CleanupPerfMetrics(retentionDays int) error {
	if LOG_DB == nil || retentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	return LOG_DB.Where("bucket_start < ?", cutoff).Delete(&PerfMetricBucket{}).Error
}

func ResetPerfMetricsForTest() {
	perfMetricsMu.Lock()
	pendingPerfMetrics = make(map[perfMetricKey]perfAccumulator)
	perfMetricsMu.Unlock()
}

func takePendingPerfMetrics() map[perfMetricKey]perfAccumulator {
	perfMetricsMu.Lock()
	defer perfMetricsMu.Unlock()
	if len(pendingPerfMetrics) == 0 {
		return nil
	}
	snapshot := pendingPerfMetrics
	pendingPerfMetrics = make(map[perfMetricKey]perfAccumulator)
	return snapshot
}

func restorePendingPerfMetrics(snapshot map[perfMetricKey]perfAccumulator) {
	if len(snapshot) == 0 {
		return
	}
	perfMetricsMu.Lock()
	defer perfMetricsMu.Unlock()
	for key, acc := range snapshot {
		existing := pendingPerfMetrics[key]
		existing.add(acc)
		pendingPerfMetrics[key] = existing
	}
}

func snapshotPendingPerfMetrics() map[perfMetricKey]perfAccumulator {
	perfMetricsMu.Lock()
	defer perfMetricsMu.Unlock()
	if len(pendingPerfMetrics) == 0 {
		return nil
	}
	snapshot := make(map[perfMetricKey]perfAccumulator, len(pendingPerfMetrics))
	for key, acc := range pendingPerfMetrics {
		snapshot[key] = acc
	}
	return snapshot
}

func upsertPerfMetricBucket(tx *gorm.DB, key perfMetricKey, acc perfAccumulator, now int64) error {
	var bucket PerfMetricBucket
	err := tx.Where(
		"bucket_start = ? AND bucket_seconds = ? AND model_name = ? AND "+perfMetricGroupCol()+" = ?",
		key.bucketStart,
		key.bucketSeconds,
		key.modelName,
		key.group,
	).First(&bucket).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&PerfMetricBucket{
			BucketStart:       key.bucketStart,
			BucketSeconds:     key.bucketSeconds,
			ModelName:         key.modelName,
			Group:             key.group,
			RequestCount:      acc.requestCount,
			SuccessCount:      acc.successCount,
			TotalLatencyMs:    acc.totalLatencyMs,
			TotalTTFTMs:       acc.totalTTFTMs,
			TTFTCount:         acc.ttftCount,
			CompletionTokens:  acc.completionTokens,
			TotalTPSLatencyMs: acc.totalTPSLatencyMs,
			CreatedAt:         now,
			UpdatedAt:         now,
		}).Error
	}
	return tx.Model(&bucket).Updates(map[string]interface{}{
		"request_count":        gorm.Expr("request_count + ?", acc.requestCount),
		"success_count":        gorm.Expr("success_count + ?", acc.successCount),
		"total_latency_ms":     gorm.Expr("total_latency_ms + ?", acc.totalLatencyMs),
		"total_ttft_ms":        gorm.Expr("total_ttft_ms + ?", acc.totalTTFTMs),
		"ttft_count":           gorm.Expr("ttft_count + ?", acc.ttftCount),
		"completion_tokens":    gorm.Expr("completion_tokens + ?", acc.completionTokens),
		"total_tps_latency_ms": gorm.Expr("total_tps_latency_ms + ?", acc.totalTPSLatencyMs),
		"updated_at":           now,
	}).Error
}

type perfMetricAggregateRow struct {
	ModelName         string
	Group             string `gorm:"column:group_name"`
	BucketStart       int64
	BucketSeconds     int64
	RequestCount      int64
	SuccessCount      int64
	TotalLatencyMs    int64
	TotalTTFTMs       int64
	TTFTCount         int64
	CompletionTokens  int64
	TotalTPSLatencyMs int64
}

func pendingPerfMetricRows(startTimestamp int64, modelName string) []perfMetricAggregateRow {
	modelName = strings.TrimSpace(modelName)
	pending := snapshotPendingPerfMetrics()
	if len(pending) == 0 {
		return nil
	}
	rows := make([]perfMetricAggregateRow, 0)
	for key, acc := range pending {
		if key.bucketStart < startTimestamp || key.modelName != modelName {
			continue
		}
		rows = append(rows, perfMetricAggregateRow{
			ModelName:         key.modelName,
			Group:             key.group,
			BucketStart:       key.bucketStart,
			BucketSeconds:     key.bucketSeconds,
			RequestCount:      acc.requestCount,
			SuccessCount:      acc.successCount,
			TotalLatencyMs:    acc.totalLatencyMs,
			TotalTTFTMs:       acc.totalTTFTMs,
			TTFTCount:         acc.ttftCount,
			CompletionTokens:  acc.completionTokens,
			TotalTPSLatencyMs: acc.totalTPSLatencyMs,
		})
	}
	return rows
}

func GetPerfMetricsSummary(hours int, limit int) (PerfMetricsSummary, error) {
	startTimestamp := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	if limit <= 0 {
		limit = defaultPerfMetricsLimit
	}

	var rows []perfMetricAggregateRow
	err := LOG_DB.Model(&PerfMetricBucket{}).
		Select(`model_name,
			SUM(request_count) AS request_count,
			SUM(success_count) AS success_count,
			SUM(total_latency_ms) AS total_latency_ms,
			SUM(total_ttft_ms) AS total_ttft_ms,
			SUM(ttft_count) AS ttft_count,
			SUM(completion_tokens) AS completion_tokens,
			SUM(total_tps_latency_ms) AS total_tps_latency_ms`).
		Where("bucket_start >= ?", startTimestamp).
		Where("model_name <> ''").
		Group("model_name").
		Scan(&rows).Error
	if err != nil {
		return PerfMetricsSummary{}, err
	}

	aggregates := make(map[string]perfAccumulator, len(rows))
	for _, row := range rows {
		acc := accumulatorFromRow(row)
		existing := aggregates[row.ModelName]
		existing.add(acc)
		aggregates[row.ModelName] = existing
	}
	for key, acc := range snapshotPendingPerfMetrics() {
		if key.bucketStart < startTimestamp || strings.TrimSpace(key.modelName) == "" {
			continue
		}
		existing := aggregates[key.modelName]
		existing.add(acc)
		aggregates[key.modelName] = existing
	}

	models := make([]PerfModelSummary, 0, len(aggregates))
	for modelName, acc := range aggregates {
		avgLatencyMs, _, successRate, avgTps := acc.summary()
		models = append(models, PerfModelSummary{
			ModelName:    modelName,
			AvgLatencyMs: avgLatencyMs,
			SuccessRate:  successRate,
			AvgTps:       avgTps,
			RequestCount: acc.requestCount,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].RequestCount == models[j].RequestCount {
			return models[i].ModelName < models[j].ModelName
		}
		return models[i].RequestCount > models[j].RequestCount
	})
	if limit > 0 && len(models) > limit {
		models = models[:limit]
	}

	return PerfMetricsSummary{Models: models}, nil
}

func GetPerfGroupHealthSummary(hours int, intervalMinutes int) (PerfGroupHealthSummary, error) {
	if hours <= 0 {
		hours = 24
	}
	if intervalMinutes <= 0 {
		intervalMinutes = 10
	}
	intervalSeconds := int64(intervalMinutes * 60)
	bucketCount := hours * 60 / intervalMinutes
	if bucketCount <= 0 {
		bucketCount = 1
	}
	now := time.Now().Unix()
	endBucketStart := now - now%intervalSeconds
	startBucket := endBucketStart - int64(bucketCount-1)*intervalSeconds
	endExclusive := endBucketStart + intervalSeconds

	type groupedAccumulator struct {
		total  perfAccumulator
		series map[int64]*perfAccumulator
	}

	groups := make(map[string]*groupedAccumulator)
	ensureGroup := func(groupName string) *groupedAccumulator {
		groupName = strings.TrimSpace(groupName)
		if groupName == "" {
			groupName = "default"
		}
		group := groups[groupName]
		if group == nil {
			group = &groupedAccumulator{series: make(map[int64]*perfAccumulator)}
			groups[groupName] = group
		}
		return group
	}

	for groupName := range ratio_setting.GetGroupRatioCopy() {
		ensureGroup(groupName)
	}

	var rows []perfMetricAggregateRow
	groupCol := perfMetricGroupCol()
	err := LOG_DB.Model(&PerfMetricBucket{}).
		Select(`bucket_start,
			bucket_seconds,
			`+groupCol+` AS group_name,
			SUM(request_count) AS request_count,
			SUM(success_count) AS success_count,
			SUM(total_latency_ms) AS total_latency_ms,
			SUM(total_ttft_ms) AS total_ttft_ms,
			SUM(ttft_count) AS ttft_count,
			SUM(completion_tokens) AS completion_tokens,
			SUM(total_tps_latency_ms) AS total_tps_latency_ms`).
		Where("bucket_start >= ?", startBucket).
		Where("bucket_start < ?", endExclusive).
		Group("bucket_start, bucket_seconds, " + groupCol).
		Order("bucket_start ASC").
		Scan(&rows).Error
	if err != nil {
		return PerfGroupHealthSummary{}, err
	}
	for key, acc := range snapshotPendingPerfMetrics() {
		if key.bucketStart < startBucket || key.bucketStart >= endExclusive {
			continue
		}
		rows = append(rows, perfMetricAggregateRow{
			Group:             key.group,
			BucketStart:       key.bucketStart,
			BucketSeconds:     key.bucketSeconds,
			RequestCount:      acc.requestCount,
			SuccessCount:      acc.successCount,
			TotalLatencyMs:    acc.totalLatencyMs,
			TotalTTFTMs:       acc.totalTTFTMs,
			TTFTCount:         acc.ttftCount,
			CompletionTokens:  acc.completionTokens,
			TotalTPSLatencyMs: acc.totalTPSLatencyMs,
		})
	}

	for _, row := range rows {
		bucketTs := row.BucketStart - row.BucketStart%intervalSeconds
		if bucketTs < startBucket || bucketTs >= endExclusive {
			continue
		}
		group := ensureGroup(row.Group)
		acc := accumulatorFromRow(row)
		group.total.add(acc)
		bucket := group.series[bucketTs]
		if bucket == nil {
			bucket = &perfAccumulator{}
			group.series[bucketTs] = bucket
		}
		bucket.add(acc)
	}

	providerCounts, err := GetGroupProviderCounts()
	if err != nil {
		return PerfGroupHealthSummary{}, err
	}
	for groupName := range providerCounts {
		ensureGroup(groupName)
	}

	ratios := ratio_setting.GetGroupRatioCopy()
	groupNames := make([]string, 0, len(groups))
	for groupName := range groups {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)

	resultGroups := make([]PerfGroupHealth, 0, len(groupNames))
	for _, groupName := range groupNames {
		group := groups[groupName]
		avgLatencyMs, avgTTFTMs, successRate, avgTps := group.total.summary()
		ratio, ok := ratios[groupName]
		if !ok {
			ratio = 1
		}

		buckets := make([]PerfGroupHealthBucket, 0, bucketCount)
		for i := 0; i < bucketCount; i++ {
			bucketTs := startBucket + int64(i)*intervalSeconds
			acc := group.series[bucketTs]
			if acc == nil {
				buckets = append(buckets, PerfGroupHealthBucket{
					Ts:     bucketTs,
					EndTs:  bucketTs + intervalSeconds,
					Status: "empty",
				})
				continue
			}
			bucketLatencyMs, bucketTTFTMs, bucketSuccessRate, bucketTps := acc.summary()
			buckets = append(buckets, PerfGroupHealthBucket{
				Ts:           bucketTs,
				EndTs:        bucketTs + intervalSeconds,
				RequestCount: acc.requestCount,
				SuccessCount: acc.successCount,
				SuccessRate:  bucketSuccessRate,
				AvgTTFTMs:    bucketTTFTMs,
				AvgLatencyMs: bucketLatencyMs,
				AvgTps:       bucketTps,
				Status:       perfGroupHealthStatus(acc.requestCount, bucketSuccessRate),
			})
		}

		resultGroups = append(resultGroups, PerfGroupHealth{
			Group:         groupName,
			Ratio:         ratio,
			ProviderCount: providerCounts[groupName],
			RequestCount:  group.total.requestCount,
			SuccessRate:   successRate,
			AvgTTFTMs:     avgTTFTMs,
			AvgLatencyMs:  avgLatencyMs,
			AvgTps:        avgTps,
			Buckets:       buckets,
		})
	}

	return PerfGroupHealthSummary{
		WindowHours:     hours,
		IntervalMinutes: intervalMinutes,
		BucketCount:     bucketCount,
		SeriesSchema:    common.GetPerfMetricsConfig().BucketTime,
		Groups:          resultGroups,
	}, nil
}

func GetGroupProviderCounts() (map[string]int, error) {
	var channels []Channel
	if err := DB.Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}

	groupProviders := make(map[string]map[string]struct{})
	for _, channel := range channels {
		groups := channel.GetGroups()
		if len(groups) == 0 {
			groups = []string{"default"}
		}
		providerKey := fmt.Sprintf("channel:%d", channel.Id)
		if channel.ProviderID > 0 {
			providerKey = fmt.Sprintf("provider:%d", channel.ProviderID)
		}
		for _, groupName := range groups {
			groupName = strings.TrimSpace(groupName)
			if groupName == "" {
				groupName = "default"
			}
			providers := groupProviders[groupName]
			if providers == nil {
				providers = make(map[string]struct{})
				groupProviders[groupName] = providers
			}
			providers[providerKey] = struct{}{}
		}
	}

	counts := make(map[string]int, len(groupProviders))
	for groupName, providers := range groupProviders {
		counts[groupName] = len(providers)
	}
	return counts, nil
}

func GetRankingGroupPerfStats(startTime int64, endTime int64) ([]RankingGroupPerfStat, error) {
	aggregates := make(map[string]perfAccumulator)
	if LOG_DB != nil {
		var rows []perfMetricAggregateRow
		groupExpr := fmt.Sprintf("COALESCE(NULLIF(%s, ''), 'default')", perfMetricGroupCol())
		query := LOG_DB.Model(&PerfMetricBucket{}).
			Select(groupExpr + ` AS group_name,
				SUM(request_count) AS request_count,
				SUM(success_count) AS success_count,
				SUM(total_latency_ms) AS total_latency_ms,
				SUM(total_ttft_ms) AS total_ttft_ms,
				SUM(ttft_count) AS ttft_count,
				SUM(completion_tokens) AS completion_tokens,
				SUM(total_tps_latency_ms) AS total_tps_latency_ms`).
			Group(groupExpr)
		if startTime > 0 {
			query = query.Where("bucket_start >= ?", startTime)
		}
		if endTime > 0 {
			query = query.Where("bucket_start <= ?", endTime)
		}
		if err := query.Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			groupName := normalizeRankingGroupName(row.Group)
			existing := aggregates[groupName]
			existing.add(accumulatorFromRow(row))
			aggregates[groupName] = existing
		}
	}

	for key, acc := range snapshotPendingPerfMetrics() {
		if startTime > 0 && key.bucketStart < startTime {
			continue
		}
		if endTime > 0 && key.bucketStart > endTime {
			continue
		}
		groupName := normalizeRankingGroupName(key.group)
		existing := aggregates[groupName]
		existing.add(acc)
		aggregates[groupName] = existing
	}

	groups := make([]string, 0, len(aggregates))
	for group := range aggregates {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	stats := make([]RankingGroupPerfStat, 0, len(groups))
	for _, group := range groups {
		acc := aggregates[group]
		avgLatencyMs, avgTTFTMs, successRate, avgTps := acc.summary()
		stats = append(stats, RankingGroupPerfStat{
			Group:        group,
			RequestCount: acc.requestCount,
			SuccessRate:  successRate,
			AvgTTFTMs:    avgTTFTMs,
			AvgLatencyMs: avgLatencyMs,
			AvgTps:       avgTps,
		})
	}
	return stats, nil
}

func GetPerformanceMetrics(modelName string, hours int) (PerformanceMetrics, error) {
	modelName = strings.TrimSpace(modelName)
	startTimestamp := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()

	var rows []perfMetricAggregateRow
	groupCol := perfMetricGroupCol()
	err := LOG_DB.Model(&PerfMetricBucket{}).
		Select(`bucket_start,
			bucket_seconds,
			`+groupCol+` AS group_name,
			SUM(request_count) AS request_count,
			SUM(success_count) AS success_count,
			SUM(total_latency_ms) AS total_latency_ms,
			SUM(total_ttft_ms) AS total_ttft_ms,
			SUM(ttft_count) AS ttft_count,
			SUM(completion_tokens) AS completion_tokens,
			SUM(total_tps_latency_ms) AS total_tps_latency_ms`).
		Where("bucket_start >= ?", startTimestamp).
		Where("model_name = ?", modelName).
		Group("bucket_start, bucket_seconds, " + groupCol).
		Order("bucket_start ASC").
		Scan(&rows).Error
	if err != nil {
		return PerformanceMetrics{}, err
	}
	rows = append(rows, pendingPerfMetricRows(startTimestamp, modelName)...)

	type groupedAccumulator struct {
		total  perfAccumulator
		series map[int64]*perfAccumulator
	}

	groups := make(map[string]*groupedAccumulator)
	for _, row := range rows {
		groupName := strings.TrimSpace(row.Group)
		if groupName == "" {
			groupName = "default"
		}
		group, ok := groups[groupName]
		if !ok {
			group = &groupedAccumulator{series: make(map[int64]*perfAccumulator)}
			groups[groupName] = group
		}
		acc := accumulatorFromRow(row)
		group.total.add(acc)
		bucket := group.series[row.BucketStart]
		if bucket == nil {
			bucket = &perfAccumulator{}
			group.series[row.BucketStart] = bucket
		}
		bucket.add(acc)
	}

	groupNames := make([]string, 0, len(groups))
	for groupName := range groups {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)

	resultGroups := make([]PerformanceGroup, 0, len(groupNames))
	for _, groupName := range groupNames {
		group := groups[groupName]
		avgLatencyMs, avgTTFTMs, successRate, avgTps := group.total.summary()

		bucketTsList := make([]int64, 0, len(group.series))
		for bucketTs := range group.series {
			bucketTsList = append(bucketTsList, bucketTs)
		}
		sort.Slice(bucketTsList, func(i, j int) bool {
			return bucketTsList[i] < bucketTsList[j]
		})

		series := make([]PerformanceSeriesPoint, 0, len(bucketTsList))
		for _, bucketTs := range bucketTsList {
			bucketLatencyMs, bucketTTFTMs, bucketSuccessRate, bucketTps := group.series[bucketTs].summary()
			series = append(series, PerformanceSeriesPoint{
				Ts:           bucketTs,
				AvgTTFTMs:    bucketTTFTMs,
				AvgLatencyMs: bucketLatencyMs,
				SuccessRate:  bucketSuccessRate,
				AvgTps:       bucketTps,
			})
		}

		resultGroups = append(resultGroups, PerformanceGroup{
			Group:        groupName,
			AvgTTFTMs:    avgTTFTMs,
			AvgLatencyMs: avgLatencyMs,
			SuccessRate:  successRate,
			AvgTps:       avgTps,
			Series:       series,
		})
	}

	config := common.GetPerfMetricsConfig()
	return PerformanceMetrics{
		ModelName:    modelName,
		SeriesSchema: config.BucketTime,
		Groups:       resultGroups,
	}, nil
}

func accumulatorFromRow(row perfMetricAggregateRow) perfAccumulator {
	return perfAccumulator{
		requestCount:      row.RequestCount,
		successCount:      row.SuccessCount,
		totalLatencyMs:    row.TotalLatencyMs,
		totalTTFTMs:       row.TotalTTFTMs,
		ttftCount:         row.TTFTCount,
		completionTokens:  row.CompletionTokens,
		totalTPSLatencyMs: row.TotalTPSLatencyMs,
	}
}

func (a *perfAccumulator) add(other perfAccumulator) {
	a.requestCount += other.requestCount
	a.successCount += other.successCount
	a.totalLatencyMs += other.totalLatencyMs
	a.totalTTFTMs += other.totalTTFTMs
	a.ttftCount += other.ttftCount
	a.completionTokens += other.completionTokens
	a.totalTPSLatencyMs += other.totalTPSLatencyMs
}

func (a perfAccumulator) summary() (avgLatencyMs float64, avgTTFTMs float64, successRate float64, avgTps float64) {
	if a.requestCount > 0 {
		successRate = roundFloat(float64(a.successCount)/float64(a.requestCount)*100, 2)
	}
	if a.successCount > 0 {
		avgLatencyMs = roundFloat(float64(a.totalLatencyMs)/float64(a.successCount), 2)
	}
	if a.ttftCount > 0 {
		avgTTFTMs = roundFloat(float64(a.totalTTFTMs)/float64(a.ttftCount), 2)
	}
	if a.totalTPSLatencyMs > 0 {
		avgTps = roundFloat(float64(a.completionTokens)/(float64(a.totalTPSLatencyMs)/1000), 2)
	}
	return avgLatencyMs, avgTTFTMs, successRate, avgTps
}

func roundFloat(value float64, precision int) float64 {
	if precision < 0 {
		return value
	}
	multiplier := math.Pow10(precision)
	return math.Round(value*multiplier) / multiplier
}

func perfGroupHealthStatus(requestCount int64, successRate float64) string {
	if requestCount <= 0 {
		return "empty"
	}
	if successRate >= 99 {
		return "ok"
	}
	if successRate >= 95 {
		return "warning"
	}
	return "error"
}

func perfMetricGroupCol() string {
	if logGroupCol != "" {
		return logGroupCol
	}
	if common.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}
