package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRequestBodySnapshotPreservesReadSeekerPosition(t *testing.T) {
	setRequestDetailLogging(t, common.RequestDetailLogFailed)
	reader := bytes.NewReader([]byte(`{"model":"test"}`))
	_, err := reader.Seek(3, io.SeekStart)
	require.NoError(t, err)

	snapshot := requestBodySnapshot(nil, reader)
	require.Equal(t, `{"model":"test"}`, snapshot)

	pos, err := reader.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	require.Equal(t, int64(3), pos)
}

func TestRecordDetailModesAndStreamSnapshots(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.RequestDetail{}))
	previousDB := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = previousDB; _ = sqlDB.Close() })

	for _, mode := range []int32{common.RequestDetailLogOff, common.RequestDetailLogFailed, common.RequestDetailLogAll} {
		for _, failed := range []bool{false, true} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("mode=%d/failed=%t/stream=%t", mode, failed, stream), func(t *testing.T) {
					setRequestDetailLogging(t, mode)
					ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
					ctx.Set(common.RequestIdKey, t.Name())
					ctx.Set("id", 7)
					body := bytes.NewBufferString("unread response tail")
					resp := &http.Response{Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(body)}
					recordDetail := buildRecordDetailFunc(ctx, &relaycommon.RelayInfo{IsStream: stream}, "request body", &resp)
					capture := &common.RequestDetailBodyCapture{}
					response := `{"result":"ok"}`
					if stream {
						response = "data: {\"result\":\"ok\"}\n\ndata: [DONE]\n\n"
					}
					_, err := capture.Write([]byte(response))
					require.NoError(t, err)
					ctx.Set("upstream_response_body_buf", capture)
					ctx.Set("upstream_request_headers", http.Header{"Content-Type": {"application/json"}})
					recordDetail(failed)
					recordDetail(false) // The success defer must not duplicate a failed attempt.
					require.Equal(t, "unread response tail", body.String())
					query := db.Model(&model.RequestDetail{}).Where("request_id = ?", t.Name())
					if mode == common.RequestDetailLogOff || (mode == common.RequestDetailLogFailed && !failed) {
						var count int64
						require.NoError(t, query.Count(&count).Error)
						require.Zero(t, count)
						return
					}
					require.Eventually(t, func() bool {
						var count int64
						return query.Count(&count).Error == nil && count == 1
					}, time.Second, 5*time.Millisecond)
					var details []model.RequestDetail
					require.NoError(t, query.Find(&details).Error)
					require.Len(t, details, 1)
					require.Equal(t, 7, details[0].UserId)
					require.Equal(t, "request body", details[0].RequestBody)
					require.Equal(t, response, details[0].ResponseBody)
					require.JSONEq(t, `{"Content-Type":["application/json"]}`, details[0].RequestHeaders)
				})
			}
		}
	}
}

func TestRecordDetailClearsPreviousAttempt(t *testing.T) {
	setRequestDetailLogging(t, common.RequestDetailLogAll)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	keys := []string{"upstream_request_headers", "upstream_request_body", "upstream_response_headers", "upstream_response_body", "upstream_response_body_buf"}
	for _, key := range keys {
		ctx.Set(key, "previous attempt")
	}
	var resp *http.Response
	_ = buildRecordDetailFunc(ctx, nil, "new request", &resp)
	for _, key := range keys {
		value, _ := ctx.Get(key)
		require.Nil(t, value, key)
	}
}

func TestRequestBodySnapshotFallsBackToBodyStorage(t *testing.T) {
	setRequestDetailLogging(t, common.RequestDetailLogFailed)
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

func setRequestDetailLogging(t *testing.T, mode int32) {
	t.Helper()
	previous := common.LogRequestDetailMode.Swap(mode)
	t.Cleanup(func() { common.LogRequestDetailMode.Store(previous) })
}

func TestRequestBodySnapshotDisabled(t *testing.T) {
	setRequestDetailLogging(t, common.RequestDetailLogOff)

	// An opaque reader would require accessing the context to obtain body storage.
	reader := struct{ io.Reader }{bytes.NewBufferString("request body")}
	require.Empty(t, requestBodySnapshot(nil, reader))
}

func TestRecordDetailDisabledDoesNotReadResponse(t *testing.T) {
	for _, mode := range []int32{common.RequestDetailLogOff, common.RequestDetailLogFailed, common.RequestDetailLogAll} {
		name := "disabled before request"
		if mode != common.RequestDetailLogOff {
			name = "disabled during request"
		}
		t.Run(name, func(t *testing.T) {
			setRequestDetailLogging(t, mode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			body := bytes.NewBufferString("response body")
			resp := &http.Response{Body: io.NopCloser(body)}
			recordDetail := buildRecordDetailFunc(ctx, nil, "request body", &resp)

			common.LogRequestDetailMode.Store(common.RequestDetailLogOff)
			recordDetail(true)
			require.Equal(t, "response body", body.String())
		})
	}
}
