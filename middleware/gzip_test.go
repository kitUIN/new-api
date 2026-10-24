package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func encodeZstdRequestBody(t *testing.T, body []byte) []byte {
	t.Helper()
	writer, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1), zstd.WithWindowSize(1024))
	require.NoError(t, err)
	defer writer.Close()
	return writer.EncodeAll(body, nil)
}

type trackedCompressedBody struct {
	io.Reader
	closed bool
}

func (b *trackedCompressedBody) Close() error {
	b.closed = true
	return nil
}

func TestDecompressRequestMiddlewareReusableJSON(t *testing.T) {
	payload := []byte(`{"model":"test-model","messages":[{"role":"user","content":"你好"}]}`)
	for _, encoding := range []string{"", "gzip", "br", "zstd"} {
		t.Run(encoding, func(t *testing.T) {
			var compressed bytes.Buffer
			switch encoding {
			case "gzip":
				writer := gzip.NewWriter(&compressed)
				_, err := writer.Write(payload)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
			case "br":
				writer := brotli.NewWriter(&compressed)
				_, err := writer.Write(payload)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
			case "zstd":
				compressed.Write(encodeZstdRequestBody(t, payload))
			default:
				compressed.Write(payload)
			}

			router := gin.New()
			router.Use(DecompressRequestMiddleware(), BodyStorageCleanup())
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				require.Empty(t, c.GetHeader("Content-Encoding"))
				if encoding != "" {
					require.Empty(t, c.GetHeader("Content-Length"))
					require.EqualValues(t, -1, c.Request.ContentLength)
				}
				// Distribution and relay both parse the same request body.
				for i := 0; i < 2; i++ {
					var request struct {
						Model string `json:"model"`
					}
					require.NoError(t, common.UnmarshalBodyReusable(c, &request))
					require.Equal(t, "test-model", request.Model)
				}
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, payload, body)
				c.Status(http.StatusNoContent)
			})

			body := &trackedCompressedBody{Reader: bytes.NewReader(compressed.Bytes())}
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			request.Body = body
			request.ContentLength = int64(compressed.Len())
			request.Header.Set("Content-Length", strconv.Itoa(compressed.Len()))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Content-Encoding", encoding)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			require.True(t, body.closed)
		})
	}
}

func TestDecompressRequestMiddlewareInvalidZstd(t *testing.T) {
	valid := encodeZstdRequestBody(t, []byte(`{"model":"test-model"}`))
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)-1] ^= 0xff
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{name: "invalid frame", body: []byte("invalid zstd")},
		{name: "truncated frame", body: valid[:len(valid)-1]},
		{name: "invalid checksum", body: corrupt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(DecompressRequestMiddleware(), BodyStorageCleanup())
			router.POST("/v1/messages", func(c *gin.Context) {
				var request map[string]any
				err := common.UnmarshalBodyReusable(c, &request)
				require.Error(t, err)
				c.AbortWithStatus(http.StatusBadRequest)
			})
			body := &trackedCompressedBody{Reader: bytes.NewReader(tc.body)}
			request := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			request.Body = body
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Content-Encoding", "zstd")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.True(t, body.closed)
		})
	}
}

func TestDecompressRequestMiddlewareZstdSizeLimit(t *testing.T) {
	original := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 1
	t.Cleanup(func() { constant.MaxRequestBodyMB = original })
	const limit = 1 << 20
	for _, size := range []int{limit, limit + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			payload := bytes.Repeat([]byte("a"), size)
			compressed := encodeZstdRequestBody(t, payload)
			require.Less(t, len(compressed), limit)
			router := gin.New()
			router.Use(DecompressRequestMiddleware())
			router.POST("/v1/responses", func(c *gin.Context) {
				defer c.Request.Body.Close()
				body, err := io.ReadAll(c.Request.Body)
				if size > limit {
					var maxBytesError *http.MaxBytesError
					require.ErrorAs(t, err, &maxBytesError)
					require.EqualValues(t, limit, maxBytesError.Limit)
					require.Len(t, body, limit)
					c.AbortWithStatus(http.StatusRequestEntityTooLarge)
					return
				}
				require.NoError(t, err)
				require.Equal(t, payload, body)
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(compressed))
			request.Header.Set("Content-Encoding", "zstd")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if size > limit {
				require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
			} else {
				require.Equal(t, http.StatusNoContent, response.Code)
			}
		})
	}
}

func TestDecompressRequestMiddlewareZstdEarlyAbort(t *testing.T) {
	router := gin.New()
	router.Use(DecompressRequestMiddleware())
	var decodedBody io.ReadCloser
	router.POST("/v1/messages", func(c *gin.Context) {
		decodedBody = c.Request.Body
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(encodeZstdRequestBody(t, []byte("payload"))))
	request.Header.Set("Content-Encoding", "zstd")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.NotNil(t, decodedBody)
	_, err := decodedBody.Read(make([]byte, 1))
	require.ErrorIs(t, err, zstd.ErrDecoderClosed)
	require.NoError(t, decodedBody.Close())
}
