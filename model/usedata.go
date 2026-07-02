package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// QuotaData 柱状图数据
type QuotaData struct {
	Id               int    `json:"id"`
	UserID           int    `json:"user_id" gorm:"index"`
	Username         string `json:"username" gorm:"index:idx_qdt_model_user_name,priority:2;size:64;default:''"`
	ModelName        string `json:"model_name" gorm:"index:idx_qdt_model_user_name,priority:1;size:64;default:''"`
	Group            string `json:"group" gorm:"column:group;index;size:64;default:''"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_qdt_created_at,priority:2"`
	TokenUsed        int    `json:"token_used" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	CacheReadTokens  int    `json:"cache_read_tokens" gorm:"default:0"`
	CacheWriteTokens int    `json:"cache_write_tokens" gorm:"default:0"`
	Count            int    `json:"count" gorm:"default:0"`
	Quota            int    `json:"quota" gorm:"default:0"`

	PerfRequestCount     int64   `json:"perf_request_count,omitempty" gorm:"-"`
	LatencyCount         int64   `json:"latency_count,omitempty" gorm:"-"`
	TotalLatencyMs       int64   `json:"total_latency_ms,omitempty" gorm:"-"`
	TotalTTFTMs          int64   `json:"total_ttft_ms,omitempty" gorm:"-"`
	TTFTCount            int64   `json:"ttft_count,omitempty" gorm:"-"`
	PerfCompletionTokens int64   `json:"perf_completion_tokens,omitempty" gorm:"-"`
	TotalTPSLatencyMs    int64   `json:"total_tps_latency_ms,omitempty" gorm:"-"`
	AvgTTFTMs            float64 `json:"avg_ttft_ms,omitempty" gorm:"-"`
	AvgLatencyMs         float64 `json:"avg_latency_ms,omitempty" gorm:"-"`
	AvgTps               float64 `json:"avg_tps,omitempty" gorm:"-"`
}

func UpdateQuotaData() {
	for {
		if common.DataExportEnabled {
			common.SysLog("正在更新数据看板数据...")
			SaveQuotaDataCache()
		}
		time.Sleep(time.Duration(common.DataExportInterval) * time.Minute)
	}
}

var CacheQuotaData = make(map[string]*QuotaData)
var CacheQuotaDataLock = sync.Mutex{}

func logQuotaDataCache(userId int, username string, modelName string, group string, quota int, createdAt int64, tokenUsed int, promptTokens int, completionTokens int, cacheReadTokens int, cacheWriteTokens int) {
	key := fmt.Sprintf("%d-%s-%s-%s-%d", userId, username, modelName, group, createdAt)
	quotaData, ok := CacheQuotaData[key]
	if ok {
		quotaData.Count += 1
		quotaData.Quota += quota
		quotaData.TokenUsed += tokenUsed
		quotaData.PromptTokens += promptTokens
		quotaData.CompletionTokens += completionTokens
		quotaData.CacheReadTokens += cacheReadTokens
		quotaData.CacheWriteTokens += cacheWriteTokens
	} else {
		quotaData = &QuotaData{
			UserID:           userId,
			Username:         username,
			ModelName:        modelName,
			Group:            group,
			CreatedAt:        createdAt,
			Count:            1,
			Quota:            quota,
			TokenUsed:        tokenUsed,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			CacheReadTokens:  cacheReadTokens,
			CacheWriteTokens: cacheWriteTokens,
		}
	}
	CacheQuotaData[key] = quotaData
}

func LogQuotaData(userId int, username string, modelName string, group string, quota int, createdAt int64, tokenUsed int, promptTokens int, completionTokens int, cacheReadTokens int, cacheWriteTokens int) {
	// 只精确到小时
	createdAt = createdAt - (createdAt % 3600)

	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	logQuotaDataCache(userId, username, modelName, group, quota, createdAt, tokenUsed, promptTokens, completionTokens, cacheReadTokens, cacheWriteTokens)
}

func SaveQuotaDataCache() {
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	size := len(CacheQuotaData)
	// 如果缓存中有数据，就保存到数据库中
	// 1. 先查询数据库中是否有数据
	// 2. 如果有数据，就更新数据
	// 3. 如果没有数据，就插入数据
	for _, quotaData := range CacheQuotaData {
		quotaDataDB := &QuotaData{}
		DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and "+commonGroupCol+" = ? and created_at = ?",
			quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.Group, quotaData.CreatedAt).First(quotaDataDB)
		if quotaDataDB.Id > 0 {
			//quotaDataDB.Count += quotaData.Count
			//quotaDataDB.Quota += quotaData.Quota
			//DB.Table("quota_data").Save(quotaDataDB)
			increaseQuotaData(quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.Group, quotaData.Count, quotaData.Quota, quotaData.CreatedAt, quotaData.TokenUsed, quotaData.PromptTokens, quotaData.CompletionTokens, quotaData.CacheReadTokens, quotaData.CacheWriteTokens)
		} else {
			DB.Table("quota_data").Create(quotaData)
		}
	}
	CacheQuotaData = make(map[string]*QuotaData)
	common.SysLog(fmt.Sprintf("保存数据看板数据成功，共保存%d条数据", size))
}

func increaseQuotaData(userId int, username string, modelName string, group string, count int, quota int, createdAt int64, tokenUsed int, promptTokens int, completionTokens int, cacheReadTokens int, cacheWriteTokens int) {
	err := DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and "+commonGroupCol+" = ? and created_at = ?",
		userId, username, modelName, group, createdAt).Updates(map[string]interface{}{
		"count":              gorm.Expr("count + ?", count),
		"quota":              gorm.Expr("quota + ?", quota),
		"token_used":         gorm.Expr("token_used + ?", tokenUsed),
		"prompt_tokens":      gorm.Expr("prompt_tokens + ?", promptTokens),
		"completion_tokens":  gorm.Expr("completion_tokens + ?", completionTokens),
		"cache_read_tokens":  gorm.Expr("cache_read_tokens + ?", cacheReadTokens),
		"cache_write_tokens": gorm.Expr("cache_write_tokens + ?", cacheWriteTokens),
	}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("increaseQuotaData error: %s", err))
	}
}

func GetQuotaDataByUsername(username string, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = DB.Table("quota_data").Where("username = ? and created_at >= ? and created_at <= ?", username, startTime, endTime).Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataByUserId(userId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = DB.Table("quota_data").Where("user_id = ? and created_at >= ? and created_at <= ?", userId, startTime, endTime).Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataGroupByUser(startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = DB.Table("quota_data").
		Select("username, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(prompt_tokens) as prompt_tokens, sum(completion_tokens) as completion_tokens, sum(cache_read_tokens) as cache_read_tokens, sum(cache_write_tokens) as cache_write_tokens").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("username, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataGroupByGroupModel(startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = DB.Table("quota_data").
		Select(commonGroupCol+" as "+commonGroupCol+", model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(prompt_tokens) as prompt_tokens, sum(completion_tokens) as completion_tokens, sum(cache_read_tokens) as cache_read_tokens, sum(cache_write_tokens) as cache_write_tokens").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group(commonGroupCol + ", model_name").
		Find(&quotaDatas).Error
	if err != nil {
		return quotaDatas, err
	}
	err = attachGroupModelPerfStats(quotaDatas, startTime, endTime)
	return quotaDatas, err
}

func GetQuotaDataGroupByUserGroupModel(userId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = DB.Table("quota_data").
		Select(commonGroupCol+" as "+commonGroupCol+", model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(prompt_tokens) as prompt_tokens, sum(completion_tokens) as completion_tokens, sum(cache_read_tokens) as cache_read_tokens, sum(cache_write_tokens) as cache_write_tokens").
		Where("user_id = ? and created_at >= ? and created_at <= ?", userId, startTime, endTime).
		Group(commonGroupCol + ", model_name").
		Find(&quotaDatas).Error
	if err != nil {
		return quotaDatas, err
	}
	err = attachGroupModelPerfStats(quotaDatas, startTime, endTime)
	return quotaDatas, err
}

func attachGroupModelPerfStats(quotaDatas []*QuotaData, startTime int64, endTime int64) error {
	if len(quotaDatas) == 0 {
		return nil
	}
	perfStats, err := GetGroupModelPerfStats(startTime, endTime)
	if err != nil {
		return err
	}
	if len(perfStats) == 0 {
		return nil
	}
	statsByKey := make(map[groupModelPerfKey]GroupModelPerfStat, len(perfStats))
	for _, stat := range perfStats {
		statsByKey[groupModelPerfKeyFromValues(stat.Group, stat.ModelName)] = stat
	}
	for _, quotaData := range quotaDatas {
		stat, ok := statsByKey[groupModelPerfKeyFromValues(quotaData.Group, quotaData.ModelName)]
		if !ok {
			continue
		}
		quotaData.PerfRequestCount = stat.RequestCount
		quotaData.LatencyCount = stat.LatencyCount
		quotaData.TotalLatencyMs = stat.TotalLatencyMs
		quotaData.TotalTTFTMs = stat.TotalTTFTMs
		quotaData.TTFTCount = stat.TTFTCount
		quotaData.PerfCompletionTokens = stat.CompletionTokens
		quotaData.TotalTPSLatencyMs = stat.TotalTPSLatencyMs
		quotaData.AvgTTFTMs = stat.AvgTTFTMs
		quotaData.AvgLatencyMs = stat.AvgLatencyMs
		quotaData.AvgTps = stat.AvgTps
	}
	return nil
}

func GetAllQuotaDates(startTime int64, endTime int64, username string) (quotaData []*QuotaData, err error) {
	if username != "" {
		return GetQuotaDataByUsername(username, startTime, endTime)
	}
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	// only select model_name, sum(count) as count, sum(quota) as quota, model_name, created_at from quota_data group by model_name, created_at;
	//err = DB.Table("quota_data").Where("created_at >= ? and created_at <= ?", startTime, endTime).Find(&quotaDatas).Error
	err = DB.Table("quota_data").Select("model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(prompt_tokens) as prompt_tokens, sum(completion_tokens) as completion_tokens, sum(cache_read_tokens) as cache_read_tokens, sum(cache_write_tokens) as cache_write_tokens, created_at").Where("created_at >= ? and created_at <= ?", startTime, endTime).Group("model_name, created_at").Find(&quotaDatas).Error
	return quotaDatas, err
}
