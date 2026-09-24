package common

import (
	"github.com/QuantumNous/new-api/constant"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreateBodyStorageFromReaderRejectsNilReader(t *testing.T) {
	_, err := CreateBodyStorageFromReader(nil, 0, 1024)
	require.EqualError(t, err, "body reader is nil")
}

func TestGetRequestBodyHandlesNilHTTPBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = &http.Request{Header: make(http.Header)}

	storage, err := GetBodyStorage(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })

	body, err := storage.Bytes()
	require.NoError(t, err)
	require.Empty(t, body)
}

func TestGetRequestBodyRejectsMissingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	_, err := GetRequestBody(ctx)
	require.EqualError(t, err, "request is nil")
}

func TestRequestBodyTimingSurvivesCachedReads(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
	before := time.Now()
	storage, err := GetBodyStorage(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })
	received := GetContextKeyTime(ctx, constant.ContextKeyRequestBodyReceivedTime)
	require.False(t, received.Before(before))
	require.False(t, received.After(time.Now()))
	_, err = GetBodyStorage(ctx)
	require.NoError(t, err)
	require.Equal(t, received, GetContextKeyTime(ctx, constant.ContextKeyRequestBodyReceivedTime))
}

type failingTimingBody struct{}

func (failingTimingBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (failingTimingBody) Close() error             { return nil }

func TestRequestBodyTimingOmitsIncompleteBody(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	ctx.Request.Body = failingTimingBody{}
	_, err := GetBodyStorage(ctx)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.True(t, GetContextKeyTime(ctx, constant.ContextKeyRequestBodyReceivedTime).IsZero())
}
