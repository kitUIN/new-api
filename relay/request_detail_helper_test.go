package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestBodySnapshotPreservesReadSeekerPosition(t *testing.T) {
	setRequestDetailLogging(t, true)
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
	setRequestDetailLogging(t, true)
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

func setRequestDetailLogging(t *testing.T, enabled bool) {
	t.Helper()
	previous := common.LogRequestDetailEnabled.Swap(enabled)
	t.Cleanup(func() { common.LogRequestDetailEnabled.Store(previous) })
}

func TestRequestBodySnapshotDisabled(t *testing.T) {
	setRequestDetailLogging(t, false)

	// An opaque reader would require accessing the context to obtain body storage.
	reader := struct{ io.Reader }{bytes.NewBufferString("request body")}
	require.Empty(t, requestBodySnapshot(nil, reader))
}

func TestRecordDetailDisabledDoesNotReadResponse(t *testing.T) {
	for _, initiallyEnabled := range []bool{false, true} {
		name := "disabled before request"
		if initiallyEnabled {
			name = "disabled during request"
		}
		t.Run(name, func(t *testing.T) {
			setRequestDetailLogging(t, initiallyEnabled)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			body := bytes.NewBufferString("response body")
			resp := &http.Response{Body: io.NopCloser(body)}
			recordDetail := buildRecordDetailFunc(ctx, nil, "request body", &resp)

			common.LogRequestDetailEnabled.Store(false)
			recordDetail()
			require.Equal(t, "response body", body.String())
		})
	}
}
