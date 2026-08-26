package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type RankingQuotaTotal struct {
	ModelName   string `json:"model_name"`
	TotalTokens int64  `json:"total_tokens"`
}

type RankingQuotaBucket struct {
	ModelName string `json:"model_name"`
	Bucket    int64  `json:"bucket"`
	Tokens    int64  `json:"tokens"`
}

type RankingUserTotal struct {
	UserID      int   `json:"user_id" gorm:"column:user_id"`
	TotalTokens int64 `json:"total_tokens" gorm:"column:total_tokens"`
	TotalQuota  int64 `json:"total_quota" gorm:"column:total_quota"`
}

type RankingGroupTotal struct {
	Group       string `json:"group" gorm:"column:group_name"`
	TotalTokens int64  `json:"total_tokens" gorm:"column:total_tokens"`
	TotalQuota  int64  `json:"total_quota" gorm:"column:total_quota"`
}

const (
	RankingUserMetricTokens = "tokens"
	RankingUserMetricQuota  = "quota"
)

func GetRankingQuotaTotals(startTime int64, endTime int64) ([]RankingQuotaTotal, error) {
	var rows []RankingQuotaTotal
	query := DB.Table("quota_data").
		Select("model_name, sum(token_used) as total_tokens").
		Where("model_name <> ''").
		Group("model_name").
		Having("sum(token_used) > 0").
		Order("total_tokens DESC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

func GetRankingQuotaBuckets(startTime int64, endTime int64, bucketSize int64) ([]RankingQuotaBucket, error) {
	if bucketSize <= 0 {
		bucketSize = 3600
	}
	bucketExpr := rankingBucketExpr(bucketSize, startTime)
	var rows []RankingQuotaBucket
	query := DB.Table("quota_data").
		Select(fmt.Sprintf("model_name, %s as bucket, sum(token_used) as tokens", bucketExpr)).
		Where("model_name <> ''").
		Group(fmt.Sprintf("model_name, %s", bucketExpr)).
		Having("sum(token_used) > 0").
		Order("bucket ASC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

func GetRankingUserTotals(startTime int64, endTime int64, limit int) ([]RankingUserTotal, error) {
	return GetRankingUserTotalsByMetric(startTime, endTime, limit, RankingUserMetricTokens)
}

func GetRankingUserTotalsByMetric(startTime int64, endTime int64, limit int, metric string) ([]RankingUserTotal, error) {
	metric = normalizeRankingUserMetric(metric)
	metricColumn := rankingUserMetricColumn(metric)
	var rows []RankingUserTotal
	query := rankingUserTotalsQuery(startTime, endTime).
		Having(rankingUserMetricHaving(metric)).
		Order(metricColumn + " DESC").
		Order("user_id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&rows).Error
	return rows, err
}

func GetRankingUserTotal(userID int, startTime int64, endTime int64) (RankingUserTotal, error) {
	if userID <= 0 {
		return RankingUserTotal{}, nil
	}
	var rows []RankingUserTotal
	err := rankingUserTotalsQuery(startTime, endTime).
		Where("user_id = ?", userID).
		Limit(1).
		Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return RankingUserTotal{UserID: userID}, err
	}
	return rows[0], nil
}

func GetRankingUserRank(userID int, totalTokens int64, startTime int64, endTime int64) (int, error) {
	return GetRankingUserRankByMetric(userID, totalTokens, startTime, endTime, RankingUserMetricTokens)
}

func GetRankingUserRankByMetric(userID int, totalMetric int64, startTime int64, endTime int64, metric string) (int, error) {
	if userID <= 0 || totalMetric <= 0 {
		return 0, nil
	}
	metric = normalizeRankingUserMetric(metric)
	metricColumn := rankingUserMetricColumn(metric)
	subQuery := rankingUserTotalsQuery(startTime, endTime).
		Having(rankingUserMetricHaving(metric))
	var count int64
	err := DB.Table("(?) AS ranked", subQuery).
		Where(fmt.Sprintf("ranked.%s > ? OR (ranked.%s = ? AND ranked.user_id < ?)", metricColumn, metricColumn), totalMetric, totalMetric, userID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count) + 1, nil
}

func GetRankingGroupTotals(startTime int64, endTime int64, limit int) ([]RankingGroupTotal, error) {
	groupExpr := rankingGroupExpr()
	var rows []RankingGroupTotal
	query := DB.Table("quota_data").
		Select(fmt.Sprintf("%s AS group_name, sum(token_used) as total_tokens, sum(quota) as total_quota", groupExpr)).
		Group(groupExpr).
		Having("sum(token_used) > 0").
		Order("total_tokens DESC").
		Order("group_name ASC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&rows).Error
	for i := range rows {
		rows[i].Group = normalizeRankingGroupName(rows[i].Group)
	}
	return rows, err
}

func rankingUserTotalsQuery(startTime int64, endTime int64) *gorm.DB {
	query := DB.Table("quota_data").
		Select("user_id, sum(token_used) as total_tokens, sum(quota) as total_quota").
		Where("user_id > 0").
		Group("user_id")
	return applyRankingQuotaTimeRange(query, startTime, endTime)
}

func normalizeRankingUserMetric(metric string) string {
	switch metric {
	case RankingUserMetricQuota:
		return RankingUserMetricQuota
	default:
		return RankingUserMetricTokens
	}
}

func rankingUserMetricColumn(metric string) string {
	if normalizeRankingUserMetric(metric) == RankingUserMetricQuota {
		return "total_quota"
	}
	return "total_tokens"
}

func rankingUserMetricHaving(metric string) string {
	if normalizeRankingUserMetric(metric) == RankingUserMetricQuota {
		return "sum(quota) > 0"
	}
	return "sum(token_used) > 0"
}

func rankingBucketExpr(bucketSize int64, bucketOrigin int64) string {
	if common.UsingMySQL {
		return fmt.Sprintf("FLOOR((created_at - %d) / %d) * %d + %d", bucketOrigin, bucketSize, bucketSize, bucketOrigin)
	}
	return fmt.Sprintf("((created_at - %d) / %d) * %d + %d", bucketOrigin, bucketSize, bucketSize, bucketOrigin)
}

func rankingGroupExpr() string {
	return fmt.Sprintf("COALESCE(NULLIF(%s, ''), 'default')", commonGroupCol)
}

func normalizeRankingGroupName(group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		return "default"
	}
	return group
}

func applyRankingQuotaTimeRange(query *gorm.DB, startTime int64, endTime int64) *gorm.DB {
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	return query
}
