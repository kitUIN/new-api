package common

import (
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// RequestBodyInfo describes the encoded payload, excluding headers and transport
// framing. A missing size means the length is unknown and the read is incomplete.
type RequestBodyInfo struct {
	Size       *int64 `json:"size,omitempty"`
	Encoding   string `json:"encoding"`
	RawSize    *int64 `json:"raw_size,omitempty"`
	ImageCount *int   `json:"image_count,omitempty"`
}

// CountRequestImages counts input image parts without decoding base64 or fetching
// remote URLs. Only request content containers are visited: tool schemas and
// arbitrary text mentioning images must not be counted as attachments.
func CountRequestImages(data []byte) *int {
	if !gjson.ValidBytes(data) {
		return nil
	}
	root := gjson.ParseBytes(data)
	if !root.IsObject() {
		return nil
	}
	count := 0
	var visit func(gjson.Result)
	visit = func(value gjson.Result) {
		if value.IsArray() {
			value.ForEach(func(_, item gjson.Result) bool { visit(item); return true })
			return
		}
		if !value.IsObject() {
			return
		}
		switch value.Get("type").Str {
		case "image_url", "input_image", "image":
			count++
			return
		}
		value.ForEach(func(key, part gjson.Result) bool {
			switch key.Str {
			case "content", "parts", "output":
				visit(part)
			case "inlineData", "inline_data", "fileData", "file_data":
				mimeType := part.Get("mimeType").Str
				if mimeType == "" {
					mimeType = part.Get("mime_type").Str
				}
				if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
					count++
				}
			}
			return true
		})
	}
	root.ForEach(func(key, value gjson.Result) bool {
		switch key.Str {
		case "messages", "input", "contents", "system", "systemInstruction", "system_instruction":
			visit(value)
		case "image", "images", "image_url", "mask":
			if value.IsArray() {
				value.ForEach(func(_, image gjson.Result) bool {
					if image.IsObject() || (image.Type == gjson.String && image.Str != "") {
						count++
					}
					return true
				})
			} else if value.IsObject() || (value.Type == gjson.String && value.Str != "") {
				count++
			}
		}
		return true
	})
	return &count
}

func recordClientJSONImages(c *gin.Context, data []byte) {
	if info := GetRequestBodyInfo(c, constant.ContextKeyClientRequestBodyInfo); info != nil && info.ImageCount == nil {
		info.ImageCount = CountRequestImages(data)
		SetContextKey(c, constant.ContextKeyClientRequestBodyInfo, *info)
	}
}

func recordClientMultipartImages(c *gin.Context, form *multipart.Form) {
	info := GetRequestBodyInfo(c, constant.ContextKeyClientRequestBodyInfo)
	if info == nil || info.ImageCount != nil {
		return
	}
	count := 0
	for name, files := range form.File {
		for _, file := range files {
			if strings.HasPrefix(strings.ToLower(file.Header.Get("Content-Type")), "image/") ||
				name == "image" || name == "image[]" || name == "images" || name == "images[]" || name == "mask" {
				count++
			}
		}
	}
	info.ImageCount = &count
	SetContextKey(c, constant.ContextKeyClientRequestBodyInfo, *info)
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
