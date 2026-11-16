package model

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const pendingLogContextKey = "pending_usage_log"

// A context has one active request (or one Responses WebSocket round). Updates
// use the database ID, never a client-supplied request ID, to isolate requests.
type pendingUsageLog struct {
	log       Log
	finished  bool
	attempts  []map[string]interface{}
	lastError *Log
}

var activeUsageLogs sync.Map // log ID -> user ID; only IDs are retained for leases

func pendingLog(c *gin.Context) *pendingUsageLog {
	if c == nil {
		return nil
	}
	value, _ := c.Get(pendingLogContextKey)
	pending, _ := value.(*pendingUsageLog)
	return pending
}

func BeginRequestLog(c *gin.Context, info *relaycommon.RelayInfo, other map[string]interface{}) {
	if !common.LogConsumeEnabled || LOG_DB == nil || info == nil || info.IsCountTokens || info.ResponsesWebSocketWarmup {
		return
	}
	now := time.Now().Unix()
	log := Log{
		UserId: info.UserId, Username: c.GetString("username"),
		TokenId: info.TokenId, TokenName: c.GetString("token_name"),
		ModelName: info.OriginModelName, Group: info.UsingGroup,
		CreatedAt: now, HeartbeatAt: now, Type: LogTypePending, Status: "in_progress",
		RequestId: info.RequestId, IsStream: info.IsStream, Ip: c.ClientIP(),
		Other: common.MapToJsonStr(appendRequestLogInfo(c, other)),
	}
	if !info.StartTime.IsZero() {
		log.CreatedAt = info.StartTime.Unix()
	}
	if info.ChannelMeta != nil {
		log.ChannelId = info.ChannelId
	}
	// The first WebSocket round adopts the placeholder created before dialing
	// the upstream. Later rounds create their own rows after the previous one ends.
	if pending := pendingLog(c); pending != nil && !pending.finished {
		log.Id, log.CreatedAt = pending.log.Id, pending.log.CreatedAt
		if err := LOG_DB.Model(&Log{}).Where("id = ? AND user_id = ?", log.Id, log.UserId).
			Select("request_id", "model_name", "group", "channel_id", "is_stream", "other").Updates(&log).Error; err != nil {
			common.SysError("failed to update usage log round: " + err.Error())
		}
		pending.log = log
		NotifyUsageLogChanged(log.UserId)
		return
	}
	if err := LOG_DB.Create(&log).Error; err != nil {
		common.SysError("failed to start usage log: " + err.Error())
		return
	}
	c.Set(pendingLogContextKey, &pendingUsageLog{log: log})
	activeUsageLogs.Store(log.Id, log.UserId)
	NotifyUsageLogChanged(log.UserId)
}

// Keep retries as details of the running request instead of completing it on
// the first failed channel. Bound retained errors for long failover chains.
func recordPendingLogAttempt(c *gin.Context, log *Log) bool {
	pending := pendingLog(c)
	if pending == nil || pending.finished || pending.log.UserId != log.UserId {
		return false
	}
	pending.lastError = log
	if len(pending.attempts) < 20 {
		pending.attempts = append(pending.attempts, map[string]interface{}{
			"channel": log.ChannelId, "message": log.Content, "created_at": log.CreatedAt,
		})
	}
	return true
}

func appendPendingAttempts(pending *pendingUsageLog, log *Log) {
	if len(pending.attempts) == 0 {
		return
	}
	other := make(map[string]interface{})
	_ = common.UnmarshalJsonStr(log.Other, &other)
	if other == nil {
		other = make(map[string]interface{})
	}
	admin, _ := other["admin_info"].(map[string]interface{})
	if admin == nil {
		admin = make(map[string]interface{})
	}
	admin["retry_errors"] = pending.attempts
	other["admin_info"] = admin
	log.Other = common.MapToJsonStr(other)
}

func completePendingLog(c *gin.Context, log *Log) (bool, error) {
	pending := pendingLog(c)
	if pending == nil || pending.log.UserId != log.UserId || pending.log.RequestId != log.RequestId {
		return false, nil
	}
	if pending.finished {
		return true, nil
	}
	log.Id = pending.log.Id
	log.CreatedAt = pending.log.CreatedAt
	log.FinishedAt = time.Now().Unix()
	log.HeartbeatAt = 0
	if log.Status == "" {
		log.Status = "completed"
		var other struct {
			StreamStatus *struct {
				Status string `json:"status"`
			} `json:"stream_status"`
		}
		_ = common.UnmarshalJsonStr(log.Other, &other)
		if other.StreamStatus != nil && other.StreamStatus.Status != "ok" {
			log.Status = "interrupted"
		}
	}
	appendPendingAttempts(pending, log)
	// Select all writable fields so explicit zero usage/false values replace
	// the placeholder. A late completion may repair an expired lease.
	err := LOG_DB.Model(&Log{}).Where("id = ? AND user_id = ?", log.Id, log.UserId).
		Select("*").Omit("id", "channel_name", "qq_id").Updates(log).Error
	if err == nil {
		pending.finished = true
	}
	activeUsageLogs.Delete(log.Id)
	return true, err
}

// FinishRequestLog is the deferred fallback for early failures, disconnects and
// panics. Normal consumption already replaced the placeholder during settlement.
func FinishRequestLog(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) {
	pending := pendingLog(c)
	if pending == nil || pending.finished || info == nil || pending.log.RequestId != info.RequestId {
		return
	}
	log := pending.log
	if pending.lastError != nil {
		log = *pending.lastError
	}
	log.RequestId = info.RequestId
	log.Type = LogTypeError
	log.Status = "interrupted"
	log.Content = "Request interrupted before completion"
	if apiErr != nil {
		log.Status = "failed"
		log.Content = apiErr.MaskSensitiveErrorWithStatusCode()
	}
	if info.ChannelMeta != nil {
		log.ChannelId = info.ChannelId
	}
	log.Group = info.UsingGroup
	log.UseTime = int(time.Since(info.StartTime).Seconds())
	if _, err := completePendingLog(c, &log); err != nil {
		common.SysError("failed to finish usage log: " + err.Error())
	} else {
		NotifyUsageLogChanged(log.UserId)
	}
}

// Non-billable task submissions can finish successfully without ever calling
// RecordConsumeLog. Remove their temporary row instead of reporting a failure.
func DiscardRequestLog(c *gin.Context, info *relaycommon.RelayInfo) {
	pending := pendingLog(c)
	if pending == nil || pending.finished || info == nil || pending.log.RequestId != info.RequestId {
		return
	}
	if err := LOG_DB.Where("id = ? AND user_id = ? AND type = ?", pending.log.Id, pending.log.UserId, LogTypePending).
		Delete(&Log{}).Error; err != nil {
		common.SysError("failed to remove non-billable usage placeholder: " + err.Error())
		return
	}
	pending.finished = true
	activeUsageLogs.Delete(pending.log.Id)
	NotifyUsageLogChanged(pending.log.UserId)
}

func maintainUsageLogLeases(now int64) error {
	ids := make([]int, 0)
	activeUsageLogs.Range(func(key, _ interface{}) bool {
		ids = append(ids, key.(int))
		return true
	})
	// Keep each statement below SQLite's parameter limit.
	for start := 0; start < len(ids); start += 500 {
		end := min(start+500, len(ids))
		if err := LOG_DB.Model(&Log{}).Where("id IN ? AND type = ?", ids[start:end], LogTypePending).
			Update("heartbeat_at", now).Error; err != nil {
			return err
		}
	}
	// Shared leases also cover requests on other nodes. Never clear every
	// pending row on startup, which would interrupt healthy nodes' requests.
	return LOG_DB.Model(&Log{}).Where("type = ? AND heartbeat_at < ?", LogTypePending, now-120).
		Updates(map[string]interface{}{
			"type": LogTypeError, "status": "interrupted", "finished_at": now,
			"content": "Request worker stopped reporting progress", "heartbeat_at": 0,
		}).Error
}

func StartUsageLogMaintenance(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := maintainUsageLogLeases(time.Now().Unix()); err != nil {
					common.SysError("usage log lease maintenance failed: " + err.Error())
				}
			}
		}
	}()
	startUsageLogNotifications(ctx)
}
