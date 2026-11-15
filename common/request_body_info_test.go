package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
