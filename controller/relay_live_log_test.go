package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelayUsageLogVisibleWhileUpstreamIsStillRunning(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.User{}, &model.Channel{}, &model.PerfMetricBucket{}))
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldCount, oldConsume, oldRetry := constant.CountToken, common.LogConsumeEnabled, common.RetryTimes
	oldRatios := ratio_setting.ModelRatio2JSONString()
	oldFree := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	model.DB, model.LOG_DB = db, db
	constant.CountToken, common.LogConsumeEnabled, common.RetryTimes = false, true, 0
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"live-log-test":0}`))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		constant.CountToken, common.LogConsumeEnabled, common.RetryTimes = oldCount, oldConsume, oldRetry
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = oldFree
		_ = ratio_setting.UpdateModelRatioByJSONString(oldRatios)
		_ = sqlDB.Close()
	})
	if service.GetHttpClient() == nil {
		service.InitHttpClient()
	}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-1","model":"live-log-test","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`))
	}))
	defer server.Close()
	defer once.Do(func() { close(release) })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"live-log-test","messages":[{"role":"user","content":"hello"}]}`)).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "live-log-request")
	c.Set("id", 42)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "live-log-test")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyIsPlayground, true)
	done := make(chan struct{})
	go func() { defer close(done); Relay(c, types.RelayFormatOpenAI) }()
	select {
	case <-started:
	case <-done:
		t.Fatal("relay ended before upstream call")
	case <-ctx.Done():
		t.Fatal("upstream did not start")
	}
	var log model.Log
	require.NoError(t, db.First(&log).Error)
	require.Equal(t, model.LogTypePending, log.Type)
	require.Equal(t, "in_progress", log.Status)
	require.Zero(t, log.Quota)
	id := log.Id
	once.Do(func() { close(release) })
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("relay did not finish")
	}
	var logs []model.Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, id, logs[0].Id)
	require.Equal(t, "completed", logs[0].Status)
	require.Equal(t, 10, logs[0].PromptTokens)
	require.Equal(t, 2, logs[0].CompletionTokens)
}
