package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
