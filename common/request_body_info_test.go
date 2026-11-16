package common

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCountRequestImages(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"text only", `{"messages":[{"content":"image_url: https://example.com/a.png"}],"tools":[{"type":"image","input_schema":{"type":"image"}}],"n":4}`, 0},
		{"chat", `{"messages":[{"content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"data:image/png;base64,YQ=="}},{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}]}`, 2},
		{"responses", `{"input":[{"content":[{"type":"input_image","file_id":"file-1"},{"type":"input_image","image_url":"https://example.com/a.png"}]},{"type":"function_call_output","output":[{"type":"input_image","image_url":"data:image/png;base64,YQ=="}]}]}`, 3},
		{"claude tool results", `{"messages":[{"content":[{"type":"image","source":{"type":"base64","data":"YQ=="}},{"type":"tool_result","content":[{"type":"image","source":{"type":"url","url":"https://example.com/a.png"}}]}]}]}`, 2},
		{"gemini", `{"contents":[{"parts":[{"inlineData":{"mimeType":"image/png","data":"YQ=="}},{"file_data":{"mime_type":"image/jpeg","file_uri":"gs://example/image"}},{"inlineData":{"mimeType":"audio/wav","data":"YQ=="}}]}]}`, 2},
		{"image edit", `{"images":[{"image_url":"https://example.com/a.png"},"data:image/png;base64,YQ=="],"mask":"https://example.com/mask.png","n":5}`, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := CountRequestImages([]byte(tc.body))
			require.NotNil(t, count)
			require.Equal(t, tc.want, *count)
		})
	}
	require.Nil(t, CountRequestImages([]byte(`{"input":`)))
}

func TestClientRequestImageInfoSurvivesRepeatedReads(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := `{"messages":[{"content":[{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}]}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	TrackRequestBody(ctx, ctx.Request, constant.ContextKeyClientRequestBodyInfo)
	t.Cleanup(func() { CleanupBodyStorage(ctx) })
	for i := 0; i < 3; i++ {
		var request map[string]interface{}
		require.NoError(t, UnmarshalBodyReusable(ctx, &request))
		info := GetRequestBodyInfo(ctx, constant.ContextKeyClientRequestBodyInfo)
		require.EqualValues(t, len(body), *info.RawSize)
		require.Equal(t, 1, *info.ImageCount)
	}
}

func TestClientMultipartImageInfo(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range []string{"image[]", "mask", "audio"} {
		part, err := writer.CreateFormFile(name, "file.bin")
		require.NoError(t, err)
		_, err = part.Write([]byte("payload"))
		require.NoError(t, err)
	}
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="attachment"; filename="photo.png"`},
		"Content-Type":        {"image/png"},
	})
	require.NoError(t, err)
	_, err = part.Write([]byte("image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	TrackRequestBody(ctx, ctx.Request, constant.ContextKeyClientRequestBodyInfo)
	t.Cleanup(func() { CleanupBodyStorage(ctx) })
	for i := 0; i < 2; i++ {
		form, err := ParseMultipartFormReusable(ctx)
		require.NoError(t, err)
		require.NoError(t, form.RemoveAll())
		info := GetRequestBodyInfo(ctx, constant.ContextKeyClientRequestBodyInfo)
		require.EqualValues(t, body.Len(), *info.RawSize)
		require.Equal(t, 3, *info.ImageCount)
	}
}

func TestTrackRequestBody(t *testing.T) {
	for _, length := range []int64{-1, 0, 6} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("你好"))
		req.ContentLength = length
		TrackRequestBody(c, req, constant.ContextKeyClientRequestBodyInfo)
		before := GetRequestBodyInfo(c, constant.ContextKeyClientRequestBodyInfo)
		require.Equal(t, "identity", before.Encoding)
		if length > 0 {
			require.EqualValues(t, 6, *before.Size)
		} else {
			require.Nil(t, before.Size, "unknown lengths must not be reported as zero")
		}
		data, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, "你好", string(data))
		require.NoError(t, req.Body.Close())
		after := GetRequestBodyInfo(c, constant.ContextKeyClientRequestBodyInfo)
		require.EqualValues(t, 6, *after.Size, "count bytes, not characters")
		if length <= 0 {
			require.Nil(t, before.Size, "snapshots must not change after reading")
		}
	}
}

func TestTrackRequestBodyEmptyAndMissing(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.Nil(t, GetRequestBodyInfo(c, constant.ContextKeyUpstreamRequestBodyInfo))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	TrackRequestBody(c, req, constant.ContextKeyUpstreamRequestBodyInfo)
	info := GetRequestBodyInfo(c, constant.ContextKeyUpstreamRequestBodyInfo)
	require.EqualValues(t, 0, *info.Size)
	require.Equal(t, http.NoBody, req.Body)
}
