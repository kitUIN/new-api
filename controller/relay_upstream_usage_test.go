package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRelayReturnsUpstreamUsageExhaustedMessage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.UpstreamUsageProvider{}, &model.UpstreamUsageAccount{}, &model.Channel{}, &model.Ability{}))
	oldDB, oldRetry, oldCache := model.DB, common.RetryTimes, common.MemoryCacheEnabled
	oldCount, oldLogs := constant.CountToken, constant.ErrorLogEnabled
	oldRatios := ratio_setting.ModelRatio2JSONString()
	oldFreePreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	t.Cleanup(func() {
		model.DB, common.RetryTimes, common.MemoryCacheEnabled = oldDB, oldRetry, oldCache
		constant.CountToken, constant.ErrorLogEnabled = oldCount, oldLogs
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = oldFreePreConsume
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldRatios))
		_ = sqlDB.Close()
	})
	model.DB, common.MemoryCacheEnabled = db, false
	constant.CountToken, constant.ErrorLogEnabled = false, false
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"usage-test":0}`))
	if service.GetHttpClient() == nil {
		service.InitHttpClient()
	}
	upstreamCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"type":"api_error","message":"No available accounts: no available accounts"}}`))
	}))
	defer server.Close()
	reset := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	provider := model.UpstreamUsageProvider{BaseURL: server.URL, IntervalMinutes: 5}
	require.NoError(t, db.Create(&provider).Error)
	usage, err := common.Marshal(service.UpstreamUsageData{SevenDay: &service.UpstreamUsageWindow{Utilization: common.GetPointer(float64(100)), ResetsAt: reset.Format(time.RFC3339)}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.UpstreamUsageAccount{ProviderID: provider.ID, AccountID: 1, GroupsJSON: `["exhausted"]`, UsageJSON: string(usage), FetchedAt: time.Now().Unix()}).Error)

	for _, tt := range []struct {
		name, path, body string
		format           types.RelayFormat
		retries          int
	}{
		{"openai", "/v1/chat/completions", `{"model":"usage-test","messages":[{"role":"user","content":"hello"}]}`, types.RelayFormatOpenAI, 0},
		{"openai after retry has no channels", "/v1/chat/completions", `{"model":"usage-test","messages":[{"role":"user","content":"hello"}]}`, types.RelayFormatOpenAI, 1},
		{"claude", "/v1/messages", `{"model":"usage-test","max_tokens":1,"messages":[{"role":"user","content":"hello"}]}`, types.RelayFormatClaude, 0},
		{"responses", "/v1/responses", `{"model":"usage-test","input":"hello"}`, types.RelayFormatOpenAIResponses, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			common.RetryTimes = tt.retries
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "exhausted")
			common.SetContextKey(c, constant.ContextKeyTokenGroup, "exhausted")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "usage-test")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
			common.SetContextKey(c, constant.ContextKeyChannelId, 1)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
			common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
			common.SetContextKey(c, constant.ContextKeyIsPlayground, true)
			c.Set(common.RequestIdKey, "usage-test-request")
			before := upstreamCalls
			Relay(c, tt.format)
			require.Equal(t, before+1, upstreamCalls)
			require.Equal(t, http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
			var body struct {
				Error types.OpenAIError `json:"error"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
			require.Contains(t, body.Error.Message, "限额已用光，下次重置时间："+reset.In(time.FixedZone("UTC+8", 8*3600)).Format("2006-01-02 15:04:05")+"（UTC+8）")
			require.Contains(t, body.Error.Message, "usage-test-request")
			require.NotContains(t, body.Error.Message, "No available accounts")
		})
	}
}
