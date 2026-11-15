package common

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// RequestBodyInfo describes the encoded payload, excluding headers and transport
// framing. A missing size means the length is unknown and the read is incomplete.
type RequestBodyInfo struct {
	Size     *int64 `json:"size,omitempty"`
	Encoding string `json:"encoding"`
}

type observedRequestBody struct {
	io.ReadCloser
	info     RequestBodyInfo
	read     atomic.Int64
	complete atomic.Bool
}

func (b *observedRequestBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.read.Add(int64(n))
	if err == io.EOF {
		b.complete.Store(true)
	}
	return n, err
}

// TrackRequestBody must run before client decompression or after upstream
// conversion/compression. Counting reads also supports chunked requests without
// buffering another copy of the body or counting cached reads on relay retries.
func TrackRequestBody(c *gin.Context, req *http.Request, key constant.ContextKey) {
	if c == nil || req == nil {
		return
	}
	encoding := strings.ToLower(strings.TrimSpace(req.Header.Get("Content-Encoding")))
	if encoding == "" {
		encoding = "identity"
	}
	info := RequestBodyInfo{Encoding: encoding}
	if req.Body == nil || req.Body == http.NoBody {
		info.Size = GetPointer(int64(0))
		SetContextKey(c, key, info)
		return
	}
	if req.ContentLength > 0 {
		info.Size = GetPointer(req.ContentLength)
	}
	body := &observedRequestBody{ReadCloser: req.Body, info: info}
	req.Body = body
	SetContextKey(c, key, body)
}

// GetRequestBodyInfo returns an immutable snapshot; HTTP transports may still be
// closing/reading a request body when an upstream error is logged.
func GetRequestBodyInfo(c *gin.Context, key constant.ContextKey) *RequestBodyInfo {
	if c == nil {
		return nil
	}
	value, _ := c.Get(string(key))
	switch body := value.(type) {
	case RequestBodyInfo:
		return &body
	case *observedRequestBody:
		info := body.info
		if body.complete.Load() {
			info.Size = GetPointer(body.read.Load())
		}
		return &info
	default:
		return nil
	}
}
