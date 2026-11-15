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
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestDoRequestRecordsFinalBodyInfo(t *testing.T) {
	service.InitHttpClient()
	previousMode := common.LogRequestDetailMode.Load()
	common.LogRequestDetailMode.Store(common.RequestDetailLogOff)
	t.Cleanup(func() { common.LogRequestDetailMode.Store(previousMode) })
	for _, tc := range []struct {
		name, compression, contentType, encoding string
		payload                                  []byte
		chunked                                  bool
	}{
		{name: "default zstd", contentType: "application/json", encoding: "zstd"},
		{name: "gzip", compression: "gzip", contentType: "application/json", encoding: "gzip"},
		{name: "disabled", compression: "none", contentType: "application/json", encoding: "identity"},
		{name: "small", contentType: "application/json", encoding: "identity", payload: []byte(`{"input":"你好"}`)},
		{name: "chunked multipart", contentType: "multipart/form-data; boundary=test", encoding: "identity", chunked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := tc.payload
			if payload == nil {
				payload = bytes.Repeat([]byte("payload"), 1024)
			}
			received := make(chan common.RequestBodyInfo, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				encoding := r.Header.Get("Content-Encoding")
				if encoding == "" {
					encoding = "identity"
				}
				received <- common.RequestBodyInfo{Size: common.GetPointer(int64(len(data))), Encoding: encoding}
				w.WriteHeader(http.StatusBadRequest) // Failed requests need the same metadata.
			}))
			defer server.Close()
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(payload))
			req, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader(payload))
			require.NoError(t, err)
			if tc.chunked {
				req.ContentLength = -1
			}
			req.Header.Set("Content-Type", tc.contentType)
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
				ChannelSetting: dto.ChannelSettings{RequestBodyCompression: tc.compression},
			}}
			resp, err := doRequest(ctx, req, info)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			got := common.GetRequestBodyInfo(ctx, constant.ContextKeyUpstreamRequestBodyInfo)
			require.NotNil(t, got)
			want := <-received
			require.Equal(t, want, *got)
			require.Equal(t, tc.encoding, got.Encoding)
			if tc.encoding != "identity" {
				require.Less(t, *got.Size, int64(len(payload)))
			} else {
				require.EqualValues(t, len(payload), *got.Size)
			}
		})
	}
}

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
