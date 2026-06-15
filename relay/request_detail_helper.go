package relay

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func buildRecordDetailFunc(c *gin.Context, info *relaycommon.RelayInfo, reqBody string, httpResp **http.Response) func() {
	requestId := c.GetString(common.RequestIdKey)
	userId := c.GetInt("id")
	var recordOnce sync.Once
	return func() {
		recordOnce.Do(func() {
			if reqBody == "" {
				if v, exists := c.Get("upstream_request_body"); exists {
					reqBody, _ = v.(string)
				}
			}
			reqHeaders := ""
			if v, exists := c.Get("upstream_request_headers"); exists {
				if h, ok := v.(http.Header); ok {
					reqHeaders = model.MarshalHeaders(h)
				}
			}
			respHeaders := ""
			if *httpResp != nil {
				respHeaders = model.MarshalHeaders((*httpResp).Header)
			} else if v, exists := c.Get("upstream_response_headers"); exists {
				if h, ok := v.(http.Header); ok {
					respHeaders = model.MarshalHeaders(h)
				}
			}
			respBody := ""
			if info == nil || !info.IsStream {
				if v, exists := c.Get("upstream_response_body"); exists {
					respBody, _ = v.(string)
				} else if v, exists := c.Get("upstream_response_body_buf"); exists {
					if buf, ok := v.(*bytes.Buffer); ok {
						respBody = buf.String()
					}
				}
			}
			if respBody == "" && *httpResp != nil && (*httpResp).Body != nil {
				if bodyBytes, readErr := io.ReadAll((*httpResp).Body); readErr == nil && len(bodyBytes) > 0 {
					respBody = string(bodyBytes)
					(*httpResp).Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}
			go model.RecordRequestDetail(requestId, userId, reqHeaders, reqBody, respHeaders, respBody)
		})
	}
}

func requestBodySnapshot(c *gin.Context, requestBody io.Reader) string {
	switch body := requestBody.(type) {
	case nil:
		return ""
	case *bytes.Buffer:
		return body.String()
	case *bytes.Reader:
		return readSeekerSnapshot(body)
	case *strings.Reader:
		return readSeekerSnapshot(body)
	case common.BodyStorage:
		return bodyStorageSnapshot(body)
	case io.ReadSeeker:
		return readSeekerSnapshot(body)
	default:
		if storage, err := common.GetBodyStorage(c); err == nil {
			return bodyStorageSnapshot(storage)
		}
		return ""
	}
}

func readSeekerSnapshot(rs io.ReadSeeker) string {
	current, err := rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return ""
	}
	if _, err = rs.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	data, readErr := io.ReadAll(rs)
	_, _ = rs.Seek(current, io.SeekStart)
	if readErr != nil {
		return ""
	}
	return string(data)
}

func bodyStorageSnapshot(storage common.BodyStorage) string {
	data, err := storage.Bytes()
	if err != nil {
		return ""
	}
	return string(data)
}
