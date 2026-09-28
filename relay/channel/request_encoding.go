package channel

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type assetResponseBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *assetResponseBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}

func upstreamAssetProtocol(req *http.Request) string {
	path := strings.TrimRight(req.URL.Path, "/")
	switch {
	case strings.HasSuffix(path, "/chat/completions"):
		return "chat"
	case strings.HasSuffix(path, "/responses"), strings.HasSuffix(path, "/responses/compact"):
		return "responses"
	case strings.HasSuffix(path, "/messages"):
		return "claude"
	case strings.Contains(path, ":generateContent"), strings.Contains(path, ":streamGenerateContent"):
		return "gemini"
	}
	return ""
}

// prepareUpstreamJSONBody runs after provider conversion and header overrides.
// Cleanup belongs to the response lifetime, not the incoming request context.
func prepareUpstreamJSONBody(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) (func(), error) {
	noop := func() {}
	media, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if req.Body == nil || info == nil || info.ChannelMeta == nil || (media != "application/json" && !strings.HasSuffix(media, "+json")) {
		return noop, nil
	}
	auth := strings.ToUpper(req.Header.Get("Authorization"))
	if info.ChannelType == constant.ChannelTypeAws || info.ChannelType == constant.ChannelTypeTencent || strings.HasPrefix(auth, "TC3-") || strings.Contains(auth, "AWS4-") || strings.Contains(auth, "HMAC-SHA256") {
		return noop, nil
	}
	compress := info.ChannelSetting.RequestBodyGzip == nil || *info.ChannelSetting.RequestBodyGzip
	rewrite := info.ChannelSetting.ResponsesImageURLs
	if !compress && !rewrite {
		return noop, nil
	}
	if req.Header.Get("Content-Encoding") != "" {
		return noop, fmt.Errorf("automatic upstream JSON encoding conflicts with Content-Encoding override")
	}
	original, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return noop, err
	}
	body := original
	release := noop
	count := 0
	if rewrite {
		body, release, count = service.RewriteRelayAssetURLs(body, service.RelayAssets, service.RelayAssetRewriteOptions{Protocol: upstreamAssetProtocol(req), ChannelType: info.ChannelType, Model: info.UpstreamModelName, UserID: info.UserId, BaseURL: service.RelayAssetBaseURL()})
	}
	encoding := "identity"
	if compress && len(body) >= 1024 {
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		if _, err = writer.Write(body); err != nil {
			release()
			return noop, err
		}
		if err = writer.Close(); err != nil {
			release()
			return noop, err
		}
		if buf.Len() < len(body) {
			body = buf.Bytes()
			encoding = "gzip"
			req.Header.Set("Content-Encoding", encoding)
		}
	}
	storage, err := common.CreateBodyStorage(body)
	if err != nil {
		release()
		return noop, err
	}
	// Each replay owns an independent storage/reader. The source storage survives
	// transport closing req.Body and is reclaimed when the upstream body closes.
	req.Body = io.NopCloser(storage)
	req.GetBody = func() (io.ReadCloser, error) {
		// Capture the final bytes independently so replay remains valid after
		// the first request body is closed by doRequest.
		data := append([]byte(nil), body...)
		return common.CreateBodyStorage(data)
	}
	req.ContentLength = int64(len(body))
	req.Header.Del("Content-Length")
	req.TransferEncoding = nil
	var once sync.Once
	cleanup := func() { once.Do(func() { storage.Close(); release() }) }
	logger.LogInfo(c, fmt.Sprintf("upstream request encoding original_bytes=%d sent_bytes=%d encoding=%s asset_urls=%d", len(original), len(body), encoding, count))
	return cleanup, nil
}
