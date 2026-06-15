package relay

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestBodySnapshotPreservesReadSeekerPosition(t *testing.T) {
	reader := bytes.NewReader([]byte(`{"model":"test"}`))
	_, err := reader.Seek(3, io.SeekStart)
	require.NoError(t, err)

	snapshot := requestBodySnapshot(nil, reader)
	require.Equal(t, `{"model":"test"}`, snapshot)

	pos, err := reader.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	require.Equal(t, int64(3), pos)
}

func TestRequestBodySnapshotFallsBackToBodyStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	storage, err := common.CreateBodyStorage([]byte(`{"messages":[]}`))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = storage.Close()
	})
	ctx.Set(common.KeyBodyStorage, storage)

	snapshot := requestBodySnapshot(ctx, struct{ io.Reader }{storage})
	require.Equal(t, `{"messages":[]}`, snapshot)
}
