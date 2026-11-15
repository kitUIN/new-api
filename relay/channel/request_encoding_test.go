package channel

import (
	"bytes"
	"compress/gzip"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestPrepareUpstreamJSONBodyCompression(t *testing.T) {
	enabled, disabled := true, false
	for _, tc := range []struct {
		name        string
		gzip        *bool
		compression string
		encoding    string
	}{
		{name: "default zstd", encoding: "zstd"},
		{name: "explicit gzip", compression: "gzip", encoding: "gzip"},
		{name: "explicit none", compression: "none"},
		{name: "explicit zstd", compression: "zstd", encoding: "zstd"},
		{name: "legacy gzip enabled", gzip: &enabled, encoding: "gzip"},
		{name: "legacy gzip disabled", gzip: &disabled},
		{name: "zstd overrides legacy disabled", gzip: &disabled, compression: "zstd", encoding: "zstd"},
		{name: "none overrides legacy gzip", gzip: &enabled, compression: "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := common.Marshal(map[string]string{"input": string(bytes.Repeat([]byte("你好 "), 1024))})
			require.NoError(t, err)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
				ChannelSetting: dto.ChannelSettings{RequestBodyGzip: tc.gzip, RequestBodyCompression: tc.compression},
			}}
			cleanup, err := prepareUpstreamJSONBody(ctx, req, info)
			require.NoError(t, err)
			t.Cleanup(cleanup)
			wireBody, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.NoError(t, req.Body.Close())
			require.Equal(t, tc.encoding, req.Header.Get("Content-Encoding"))
			require.EqualValues(t, len(wireBody), req.ContentLength)
			if tc.encoding == "" {
				require.Equal(t, payload, wireBody)
				return
			}
			require.Less(t, len(wireBody), len(payload))
			var decoded []byte
			if tc.encoding == "zstd" {
				reader, err := zstd.NewReader(nil)
				require.NoError(t, err)
				defer reader.Close()
				decoded, err = reader.DecodeAll(wireBody, nil)
				require.NoError(t, err)
			} else {
				reader, err := gzip.NewReader(bytes.NewReader(wireBody))
				require.NoError(t, err)
				defer reader.Close()
				decoded, err = io.ReadAll(reader)
				require.NoError(t, err)
			}
			require.Equal(t, payload, decoded)
			// Retries must remain valid even after the transport closes the first body.
			for i := 0; i < 2; i++ {
				replay, err := req.GetBody()
				require.NoError(t, err)
				replayed, err := io.ReadAll(replay)
				require.NoError(t, err)
				require.NoError(t, replay.Close())
				require.Equal(t, wireBody, replayed)
			}
		})
	}
}

func TestPrepareUpstreamJSONBodyZstdSkipsUnsuitableRequests(t *testing.T) {
	for _, tc := range []struct {
		name        string
		channelType int
		contentType string
		auth        string
		payload     []byte
	}{
		{name: "small JSON", contentType: "application/json", payload: []byte(`{"input":"hello"}`)},
		{name: "multipart", contentType: "multipart/form-data; boundary=test"},
		{name: "AWS signed", contentType: "application/json", channelType: constant.ChannelTypeAws},
		{name: "Tencent signed", contentType: "application/json", channelType: constant.ChannelTypeTencent},
		{name: "signed header", contentType: "application/json", auth: "AWS4-HMAC-SHA256 test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := tc.payload
			if payload == nil {
				payload = bytes.Repeat([]byte("payload"), 1024)
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(payload))
			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Authorization", tc.auth)
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
				ChannelType:    tc.channelType,
				ChannelSetting: dto.ChannelSettings{RequestBodyCompression: "zstd"},
			}}
			cleanup, err := prepareUpstreamJSONBody(ctx, req, info)
			require.NoError(t, err)
			defer cleanup()
			defer req.Body.Close()
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, payload, body)
			require.Empty(t, req.Header.Get("Content-Encoding"))
		})
	}
}

func TestPrepareUpstreamJSONBodyZstdDoesNotExpandBody(t *testing.T) {
	// Deterministic incompressible bytes also exercise the pass-through path.
	payload := make([]byte, 1024)
	_, err := rand.New(rand.NewSource(1)).Read(payload)
	require.NoError(t, err)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelSetting: dto.ChannelSettings{RequestBodyCompression: "zstd"},
	}}
	cleanup, err := prepareUpstreamJSONBody(ctx, req, info)
	require.NoError(t, err)
	defer cleanup()
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, payload, body)
	require.Empty(t, req.Header.Get("Content-Encoding"))
}

func TestPrepareUpstreamJSONBodyZstdRejectsEncodingOverride(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"input":"hello"}`)))
	defer req.Body.Close()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "br")
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelSetting: dto.ChannelSettings{RequestBodyCompression: "zstd"},
	}}
	cleanup, err := prepareUpstreamJSONBody(ctx, req, info)
	defer cleanup()
	require.ErrorContains(t, err, "Content-Encoding override")
}
