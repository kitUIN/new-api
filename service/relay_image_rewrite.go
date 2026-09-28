package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/tidwall/gjson"
)

type RelayAssetRewriteOptions struct {
	Protocol    string
	ChannelType int
	Model       string
	UserID      int
	BaseURL     string
}

type assetReplacement struct {
	start, end int
	value      []byte
}

// RewriteRelayAssetURLs visits only protocol content containers. Unrelated JSON
// bytes (including number spelling and whitespace) remain byte-for-byte intact.
func RewriteRelayAssetURLs(body []byte, cache *RelayImageCache, opt RelayAssetRewriteOptions) ([]byte, func(), int) {
	var releases []func()
	release := func() {
		for _, f := range releases {
			f()
		}
	}
	if cache == nil || opt.BaseURL == "" || !gjson.ValidBytes(body) {
		return body, release, 0
	}
	root := gjson.ParseBytes(body)
	var edits []assetReplacement
	add := func(node gjson.Result, value any) {
		encoded, err := common.Marshal(value)
		if err == nil {
			edits = append(edits, assetReplacement{node.Index, node.Index + len(node.Raw), encoded})
		}
	}
	put := func(mime, data string, file, gemini bool) string {
		maxSize := 20 << 20
		if gemini {
			maxSize = 15000000
		}
		if len(data) > (maxSize+2)/3*4 {
			return ""
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(data)
		if err != nil || len(decoded) < 64<<10 || len(decoded) > maxSize {
			return ""
		}
		mime = strings.ToLower(mime)
		ext, valid := validateRelayAsset(mime, decoded, file, gemini)
		if !valid {
			return ""
		}
		if gemini && !geminiAssetAllowed(mime, opt) {
			return ""
		}
		u, done, ok := cache.Put(opt.UserID, mime, ext, decoded, opt.BaseURL)
		if !ok {
			return ""
		}
		releases = append(releases, done)
		return u
	}
	dataURL := func(value string, file bool) string {
		if !strings.HasPrefix(value, "data:") {
			return ""
		}
		header, data, ok := strings.Cut(value[5:], ",")
		if !ok || !strings.HasSuffix(header, ";base64") {
			return ""
		}
		return put(strings.TrimSuffix(header, ";base64"), data, file, false)
	}
	var content func(gjson.Result)
	content = func(nodes gjson.Result) {
		if !nodes.IsArray() {
			return
		}
		for _, node := range nodes.Array() {
			switch opt.Protocol {
			case "chat":
				if node.Get("type").String() == "image_url" {
					field := node.Get("image_url.url")
					if u := dataURL(field.String(), false); u != "" {
						add(field, u)
					}
				}
			case "responses":
				switch node.Get("type").String() {
				case "input_image":
					field := node.Get("image_url")
					if u := dataURL(field.String(), false); u != "" {
						add(field, u)
					}
				case "input_file":
					if node.Get("file_id").Exists() || node.Get("file_url").Exists() {
						continue
					}
					if u := dataURL(node.Get("file_data").String(), true); u != "" {
						var fields map[string]json.RawMessage
						if common.Unmarshal([]byte(node.Raw), &fields) != nil {
							continue
						}
						delete(fields, "file_data")
						fields["file_url"], _ = common.Marshal(u)
						add(node, fields)
					}
				}
			case "claude":
				if opt.ChannelType != constant.ChannelTypeAnthropic {
					continue
				}
				kind := node.Get("type").String()
				if kind == "tool_result" {
					content(node.Get("content"))
					continue
				}
				source := node.Get("source")
				mime := source.Get("media_type").String()
				if source.Get("type").String() != "base64" || (kind != "image" && (kind != "document" || mime != "application/pdf")) {
					continue
				}
				if u := put(mime, source.Get("data").String(), kind == "document", false); u != "" {
					add(source, map[string]string{"type": "url", "url": u})
				}
			case "gemini":
				inline := node.Get("inlineData")
				if !inline.Exists() || node.Get("fileData").Exists() {
					continue
				}
				mime := inline.Get("mimeType").String()
				if u := put(mime, inline.Get("data").String(), true, true); u != "" {
					var fields map[string]json.RawMessage
					if common.Unmarshal([]byte(node.Raw), &fields) != nil {
						continue
					}
					delete(fields, "inlineData")
					fields["fileData"], _ = common.Marshal(map[string]string{"mimeType": mime, "fileUri": u})
					add(node, fields)
				}
			}
		}
	}
	switch opt.Protocol {
	case "chat", "claude":
		for _, message := range root.Get("messages").Array() {
			content(message.Get("content"))
		}
	case "responses":
		for _, item := range root.Get("input").Array() {
			switch item.Get("type").String() {
			case "", "message":
				content(item.Get("content"))
			case "function_call_output":
				content(item.Get("output"))
			}
		}
	case "gemini":
		for _, item := range root.Get("contents").Array() {
			content(item.Get("parts"))
		}
	}
	if len(edits) == 0 {
		release()
		return body, func() {}, 0
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var out bytes.Buffer
	pos := 0
	for _, edit := range edits {
		out.Write(body[pos:edit.start])
		out.Write(edit.value)
		pos = edit.end
	}
	out.Write(body[pos:])
	return out.Bytes(), release, len(edits)
}

func geminiAssetAllowed(mime string, opt RelayAssetRewriteOptions) bool {
	if opt.ChannelType != constant.ChannelTypeGemini && opt.ChannelType != constant.ChannelTypeVertexAi {
		return false
	}
	if !strings.Contains(strings.ToLower(opt.Model), "gemini-") || strings.Contains(strings.ToLower(opt.Model), "gemini-2.0") {
		return false
	}
	if opt.ChannelType == constant.ChannelTypeGemini && !strings.HasPrefix(opt.BaseURL, "https://") {
		return false
	}
	switch mime {
	case "image/png", "image/jpeg", "image/webp", "application/pdf", "text/plain", "video/mp4", "video/webm":
		return true
	case "audio/mpeg", "audio/wav", "audio/x-wav":
		return opt.ChannelType == constant.ChannelTypeVertexAi
	}
	return false
}

func validateRelayAsset(mime string, data []byte, file, gemini bool) (string, bool) {
	detected := http.DetectContentType(data)
	switch mime {
	case "image/png":
		return ".png", detected == mime
	case "image/jpeg":
		return ".jpg", detected == mime
	case "image/gif":
		return ".gif", detected == mime && !gemini
	case "image/webp":
		return ".webp", detected == mime
	}
	if !file {
		return "", false
	}
	switch mime {
	case "application/pdf":
		return ".pdf", bytes.HasPrefix(data, []byte("%PDF-"))
	case "text/plain", "text/markdown", "text/csv", "text/html", "text/css", "text/javascript", "application/javascript", "application/json", "application/xml", "text/xml", "text/x-python", "text/x-c", "text/x-java-source":
		return ".txt", utf8.Valid(data) && !bytes.ContainsRune(data, 0)
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx", bytes.HasPrefix(data, []byte("PK\x03\x04"))
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx", bytes.HasPrefix(data, []byte("PK\x03\x04"))
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx", bytes.HasPrefix(data, []byte("PK\x03\x04"))
	case "application/msword", "application/vnd.ms-excel", "application/vnd.ms-powerpoint":
		ext := map[string]string{"application/msword": ".doc", "application/vnd.ms-excel": ".xls", "application/vnd.ms-powerpoint": ".ppt"}[mime]
		return ext, bytes.HasPrefix(data, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	case "video/mp4":
		return ".mp4", gemini && detected == mime
	case "video/webm":
		return ".webm", gemini && detected == mime
	case "audio/mpeg":
		return ".mp3", gemini && detected == mime
	case "audio/wav", "audio/x-wav":
		return ".wav", gemini && detected == "audio/wave"
	}
	return "", false
}
