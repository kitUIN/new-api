package model

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gorm.io/gorm"
)

const pendingLogContextKey = "pending_usage_log"

const (
	logUserInterrupted    = "user_interrupted"
	logRateLimited        = "rate_limited"
	logStreamInterrupted  = "stream_interrupted"
	upstreamHTTP2Failure  = "Upstream HTTP/2 stream failed"
	upstreamStreamFailure = "Upstream response stream was interrupted"
)

func isUpstreamStreamInterruption(content string) bool {
	msg, ok := strings.CutPrefix(content, "status_code=502,")
	return ok && isUpstreamInterruptedMessage(msg)
}

func isUpstreamInterruptedMessage(msg string) bool {
	msg, _, _ = strings.Cut(msg, " (request id: ")
	msg = strings.TrimSpace(msg)
	return strings.EqualFold(msg, upstreamHTTP2Failure) || strings.EqualFold(msg, upstreamStreamFailure)
}

func RelayInterruptionStatus(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) string {
	if apiErr != nil && (apiErr.StatusCode == http.StatusBadGateway || apiErr.UpstreamStatusCode == http.StatusBadGateway) &&
		(isUpstreamInterruptedMessage(apiErr.Error()) || isUpstreamInterruptedMessage(apiErr.ToOpenAIError().Message)) {
		return logStreamInterrupted
	}
	if info != nil && info.StreamStatus != nil {
		ss := info.StreamStatus
		switch ss.EndReason {
		case relaycommon.StreamEndReasonClientGone:
			if !errors.Is(ss.EndError, context.DeadlineExceeded) {
				return logUserInterrupted
			}
			return logStreamInterrupted
		case relaycommon.StreamEndReasonUpstreamDisconnect, relaycommon.StreamEndReasonScannerErr,
			relaycommon.StreamEndReasonTimeout, relaycommon.StreamEndReasonPingFail:
			return logStreamInterrupted
		}
		if apiErr == nil && (!ss.IsNormalEnd() || ss.HasErrors()) {
			return logStreamInterrupted
		}
	}
	if (info == nil || info.ClientWs == nil) && c != nil && c.Request != nil && c.Request.Context().Err() == context.Canceled &&
		(apiErr == nil || errors.Is(apiErr, context.Canceled) || apiErr.GetErrorCode() == types.ErrorCodeDoRequestFailed) {
		return logUserInterrupted
	}
	return ""
}

// Both dashboards and routing failure counters sample the same eligible attempts.
func SkipRelayHealthStats(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) bool {
	return apiErr.IsRateLimited() || RelayInterruptionStatus(c, info, apiErr) != ""
}

func (log *Log) classifyInterruption() {
	switch log.Type {
	case LogTypeInterrupted, LogTypeError:
		if isUpstreamStreamInterruption(log.Content) {
			log.Status = logStreamInterrupted
		}
		switch log.Status {
		case "interrupted", logUserInterrupted, logStreamInterrupted:
			log.Type = LogTypeInterrupted
		default:
			if log.Content == "status_code=429" || strings.HasPrefix(log.Content, "status_code=429,") {
				log.Type, log.Status = LogTypeRateLimited, logRateLimited
			}
		}
	case LogTypeConsume:
		stream := gjson.Get(log.Other, "stream_status")
		if stream.Get("end_reason").String() == string(relaycommon.StreamEndReasonClientGone) &&
			stream.Get("end_error").String() != context.DeadlineExceeded.Error() {
			log.Status = logUserInterrupted
		} else if stream.Get("status").String() == "error" {
			log.Status = logStreamInterrupted
		}
	}
}

// Classify historical rows and retry attempts from their recorded evidence.
// Retry failures describe individual channels, even when the request succeeded.
func (log *Log) AfterFind(_ *gorm.DB) error {
	if log.Type == LogTypeError || log.Status == "" || log.Status == "interrupted" {
		log.classifyInterruption()
	}
	if !strings.Contains(log.Other, `"retry_errors"`) {
		return nil
	}
	for i, attempt := range gjson.Get(log.Other, "admin_info.retry_errors").Array() {
		if attempt.Get("status").Exists() {
			continue
		}
		legacy := Log{Type: LogTypeError, Content: attempt.Get("message").String()}
		legacy.classifyInterruption()
		if legacy.Status == "" {
			continue
		}
		path := "admin_info.retry_errors." + strconv.Itoa(i) + ".status"
		other, err := sjson.Set(log.Other, path, legacy.Status)
		if err != nil {
			return err
		}
		log.Other = other
	}
	return nil
}

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
		attempt := map[string]interface{}{
			"channel": log.ChannelId, "message": log.Content, "created_at": log.CreatedAt,
		}
		if log.Status != "" {
			attempt["status"] = log.Status
		}
		pending.attempts = append(pending.attempts, attempt)
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
	if apiErr != nil && apiErr.GetErrorCode() == types.ErrorCodeGetChannelFailed && info.LastError != nil &&
		SkipRelayHealthStats(c, info, info.LastError) {
		apiErr = info.LastError
	}
	log.Type = LogTypeError
	log.Status = "interrupted"
	log.Content = "Request interrupted before completion"
	if apiErr != nil {
		log.Status = "failed"
		log.Content = apiErr.MaskSensitiveErrorWithStatusCode()
	}
	if apiErr.IsRateLimited() {
		log.Type, log.Status = LogTypeRateLimited, logRateLimited
	} else if status := RelayInterruptionStatus(c, info, apiErr); status != "" {
		log.Status = status
	}
	log.classifyInterruption()
	if apiErr == nil {
		switch log.Status {
		case logUserInterrupted:
			log.Content = "Client disconnected before completion"
		case logStreamInterrupted:
			log.Content = "Upstream stream disconnected before completion"
		}
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
			"type": LogTypeInterrupted, "status": "interrupted", "finished_at": now,
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
