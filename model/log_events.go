package model

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const usageLogTopic = "new-api:usage-log-changed"

var usageLogEvents = struct {
	sync.Mutex
	subscribers map[chan struct{}]int
}{subscribers: make(map[chan struct{}]int)}

var usageLogPublishQueue = make(chan int, 1024)
var usageLogNodeID = common.GetRandomString(24)

type usageLogNotification struct {
	UserID int    `json:"user_id"`
	NodeID string `json:"node_id"`
}

// Notifications contain no log data. The browser re-queries the existing
// authorized, filtered endpoints. userID=0 subscribes to all users (admin only).
func SubscribeUsageLogs(userID int) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	usageLogEvents.Lock()
	usageLogEvents.subscribers[ch] = userID
	usageLogEvents.Unlock()
	return ch, func() {
		usageLogEvents.Lock()
		delete(usageLogEvents.subscribers, ch)
		usageLogEvents.Unlock()
	}
}

func notifyLocalUsageLogs(userID int) {
	usageLogEvents.Lock()
	defer usageLogEvents.Unlock()
	for ch, subscriber := range usageLogEvents.subscribers {
		if subscriber != 0 && subscriber != userID {
			continue
		}
		select {
		case ch <- struct{}{}:
		default: // coalesce updates; slow viewers cannot block model requests
		}
	}
}

func NotifyUsageLogChanged(userID int) {
	notifyLocalUsageLogs(userID)
	if common.RedisEnabled && common.RDB != nil {
		select {
		case usageLogPublishQueue <- userID:
		default: // the browser's periodic refresh repairs missed notifications
		}
	}
}

func startUsageLogNotifications(ctx context.Context) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case userID := <-usageLogPublishQueue:
				payload, err := common.Marshal(usageLogNotification{UserID: userID, NodeID: usageLogNodeID})
				if err != nil {
					continue
				}
				publishCtx, cancel := context.WithTimeout(ctx, time.Second)
				_ = common.RDB.Publish(publishCtx, usageLogTopic, string(payload)).Err()
				cancel()
			}
		}
	}()
	go func() {
		subscription := common.RDB.Subscribe(ctx, usageLogTopic)
		defer subscription.Close()
		messages := subscription.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-messages:
				if !ok {
					return
				}
				var event usageLogNotification
				if common.UnmarshalJsonStr(message.Payload, &event) == nil && event.NodeID != usageLogNodeID && event.UserID > 0 {
					notifyLocalUsageLogs(event.UserID)
				}
			}
		}
	}()
}
