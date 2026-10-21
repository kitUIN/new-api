package common

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	RequestDetailLogOff int32 = iota
	RequestDetailLogFailed
	RequestDetailLogAll
)

// The zero value keeps request/response body logging disabled by default.
var LogRequestDetailMode atomic.Int32

func ParseRequestDetailLogMode(value string) (int32, error) {
	switch value {
	case "0":
		return RequestDetailLogOff, nil
	case "1":
		return RequestDetailLogFailed, nil
	case "2":
		return RequestDetailLogAll, nil
	default:
		return RequestDetailLogOff, fmt.Errorf("request detail log mode must be 0 (off), 1 (failed), or 2 (all)")
	}
}

func IsRequestDetailLoggingEnabled() bool {
	return LogRequestDetailMode.Load() != RequestDetailLogOff
}

func ShouldRecordRequestDetail(failed bool) bool {
	mode := LogRequestDetailMode.Load()
	return mode == RequestDetailLogAll || (mode == RequestDetailLogFailed && failed)
}

// RequestDetailBodyCapture allows a stream reader and the logging snapshot to
// access captured bytes concurrently without delaying delivery to the client.
type RequestDetailBodyCapture struct {
	mu   sync.Mutex
	body bytes.Buffer
}

func (capture *RequestDetailBodyCapture) Write(p []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.body.Write(p)
}

func (capture *RequestDetailBodyCapture) String() string {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.body.String()
}
