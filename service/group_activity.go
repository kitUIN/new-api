package service

import (
	"sync"
	"time"
)

var groupLastUserRequest = struct {
	sync.RWMutex
	values map[string]time.Time
}{
	values: make(map[string]time.Time),
}

func RecordGroupUserRequest(group string, at time.Time) {
	if group == "" {
		return
	}
	groupLastUserRequest.Lock()
	groupLastUserRequest.values[group] = at
	groupLastUserRequest.Unlock()
}

func GetGroupLastUserRequest(group string) (time.Time, bool) {
	if group == "" {
		return time.Time{}, false
	}
	groupLastUserRequest.RLock()
	last, ok := groupLastUserRequest.values[group]
	groupLastUserRequest.RUnlock()
	return last, ok
}
