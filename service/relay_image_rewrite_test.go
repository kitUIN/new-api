package service

import (
	"encoding/base64"
	"github.com/tidwall/gjson"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRelayImageCacheURLAndRewrite(t *testing.T) {
	dir := t.TempDir()
	c, err := NewRelayImageCache(dir, 2<<20, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	data := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64<<10)...)
	encoded := base64.StdEncoding.EncodeToString(data)
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,` + encoded + `"}}]}]}`)
	rewritten, release, count := RewriteRelayAssetURLs(body, c, RelayAssetRewriteOptions{Protocol: "chat", UserID: 7, BaseURL: "http://example.test"})
	defer release()
	if count != 1 || strings.Contains(string(rewritten), "data:image") || !strings.Contains(string(rewritten), "http://example.test/relay-images/") {
		t.Fatalf("unexpected rewrite: count=%d body prefix=%s", count, string(rewritten[:min(len(rewritten), 120)]))
	}
	u := string(rewritten)
	assetURL := gjson.Get(u, "messages.0.content.0.image_url.url").String()
	req := httptest.NewRequest("GET", assetURL, nil)
	req.URL.Scheme = "http"
	req.URL.Host = "example.test"
	rr := httptest.NewRecorder()
	c.ServeHTTP(rr, req)
	if rr.Code != 200 || rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("serve status=%d type=%q", rr.Code, rr.Header().Get("Content-Type"))
	}
	if _, err := os.Stat(dir + "/assets.bin"); err != nil {
		t.Fatal(err)
	}
}
