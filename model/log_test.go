package model

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecordLogsPreservesGroupCombination(t *testing.T) {
	for _, logType := range []int{LogTypeConsume, LogTypeError} {
		for _, tc := range []struct {
			name        string
			combination string
			group       string
			other       map[string]interface{}
		}{
			{name: "combination", combination: "A", group: "B", other: map[string]interface{}{"group_ratio": 2}},
			{name: "failover", combination: "A", group: "C"},
			{name: "original member", combination: "A", group: "A"},
			{name: "regular group", group: "B", other: map[string]interface{}{"group_ratio": 2}},
		} {
			t.Run(tc.name+"/"+common.GetJsonString(logType), func(t *testing.T) {
				truncateTables(t)
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
				common.SetContextKey(ctx, constant.ContextKeyGroupCombination, tc.combination)
				if logType == LogTypeConsume {
					RecordConsumeLog(ctx, 42, RecordConsumeLogParams{
						TokenId: 9,
						Group:   tc.group,
						Other:   tc.other,
					})
				} else {
					RecordErrorLog(ctx, 42, 7, "test-model", "test-token", "error", 9, 1, false, tc.group, tc.other)
				}

				logs, total, err := GetUserLogs(42, logType, 0, 0, "", "", 0, 10, "", "", "")
				require.NoError(t, err)
				require.EqualValues(t, 1, total)
				require.Len(t, logs, 1)
				require.Equal(t, tc.group, logs[0].Group)
				var other map[string]interface{}
				require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
				if tc.combination != "" {
					require.Equal(t, tc.combination, other["group_combination"])
				} else {
					require.NotContains(t, other, "group_combination")
				}
				if _, ok := tc.other["group_ratio"]; ok {
					require.EqualValues(t, 2, other["group_ratio"])
				}
			})
		}
	}
}

func TestRecordLogsPreservesRequestHost(t *testing.T) {
	for _, logType := range []int{LogTypeConsume, LogTypeError} {
		for _, tc := range []struct {
			name, forwardedHost, want string
		}{
			{"direct", "", "ai.kituin.fun"},
			{"hong kong", "hk.kituin.fun", "hk.kituin.fun"},
			{"west coast", "fast.kituin.fun", "fast.kituin.fun"},
			{"multiple proxies", " hk.kituin.fun, ai.kituin.fun", "hk.kituin.fun"},
			{"empty forwarded host", "  ", "ai.kituin.fun"},
		} {
			t.Run(tc.name+"/"+common.GetJsonString(logType), func(t *testing.T) {
				truncateTables(t)
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest("POST", "https://ai.kituin.fun/v1/chat/completions", nil)
				ctx.Request.Header.Set("X-Forwarded-Host", tc.forwardedHost)
				ctx.Request.Header.Set("Origin", "https://unrelated-client.example")
				if logType == LogTypeConsume {
					RecordConsumeLog(ctx, 42, RecordConsumeLogParams{})
				} else {
					RecordErrorLog(ctx, 42, 7, "test-model", "", "error", 0, 1, false, "", nil)
				}
				logs, total, err := GetUserLogs(42, logType, 0, 0, "", "", 0, 10, "", "", "")
				require.NoError(t, err)
				require.EqualValues(t, 1, total)
				require.Len(t, logs, 1)
				var other map[string]interface{}
				require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
				require.Equal(t, tc.want, other["request_host"])
			})
		}
	}
}

func TestRecordLogsPreservesRequestBodyInfo(t *testing.T) {
	for _, logType := range []int{LogTypeConsume, LogTypeError} {
		t.Run(common.GetJsonString(logType), func(t *testing.T) {
			truncateTables(t)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			common.SetContextKey(ctx, constant.ContextKeyClientRequestBodyInfo, common.RequestBodyInfo{
				Size: common.GetPointer(int64(2048)), Encoding: "gzip",
				RawSize: common.GetPointer(int64(8192)), ImageCount: common.GetPointer(2),
			})
			common.SetContextKey(ctx, constant.ContextKeyUpstreamRequestBodyInfo, common.RequestBodyInfo{
				Size: common.GetPointer(int64(1024)), Encoding: "zstd",
			})
			if logType == LogTypeConsume {
				RecordConsumeLog(ctx, 42, RecordConsumeLogParams{})
			} else {
				RecordErrorLog(ctx, 42, 7, "test-model", "", "error", 0, 1, false, "", nil)
			}
			logs, total, err := GetUserLogs(42, logType, 0, 0, "", "", 0, 10, "", "", "")
			require.NoError(t, err)
			require.EqualValues(t, 1, total)
			var other struct {
				Client   common.RequestBodyInfo `json:"client_request_body"`
				Upstream common.RequestBodyInfo `json:"upstream_request_body"`
			}
			require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
			require.EqualValues(t, 2048, *other.Client.Size)
			require.Equal(t, "gzip", other.Client.Encoding)
			require.EqualValues(t, 8192, *other.Client.RawSize)
			require.Equal(t, 2, *other.Client.ImageCount)
			require.EqualValues(t, 1024, *other.Upstream.Size)
			require.Equal(t, "zstd", other.Upstream.Encoding)
		})
	}
}

func TestAppendRequestLogInfoClearsStaleBodyInfo(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyClientRequestBodyInfo, common.RequestBodyInfo{
		Size: common.GetPointer(int64(0)), Encoding: "identity",
	})
	other := appendRequestLogInfo(ctx, map[string]interface{}{"upstream_request_body": "previous attempt"})
	require.NotContains(t, other, "upstream_request_body")
	data, err := common.Marshal(other)
	require.NoError(t, err)
	require.JSONEq(t, `{"client_request_body":{"size":0,"encoding":"identity"}}`, string(data))
}

func TestGetUserLogsPreservesErrorDetails(t *testing.T) {
	truncateTables(t)
	rawContent := "status_code=401, invalid API key: sk-secret"
	rawOther := `{"status_code":401,"error_code":"invalid_api_key","error_type":"openai_error","channel_id":7,"admin_info":{"upstream_error_body":"secret upstream body"},"stream_status":{"status":"error"}}`
	log := &Log{
		UserId:    42,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeError,
		Content:   rawContent,
		Other:     rawOther,
		TokenId:   9,
		ChannelId: 7,
	}
	require.NoError(t, LOG_DB.Create(log).Error)

	logs, total, err := GetUserLogs(42, LogTypeError, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, log.Id, logs[0].Id)
	require.Equal(t, rawContent, logs[0].Content)
	require.JSONEq(t, rawOther, logs[0].Other)

	tokenLogs, err := GetLogByTokenId(9)
	require.NoError(t, err)
	require.Len(t, tokenLogs, 1)
	require.Equal(t, rawContent, tokenLogs[0].Content)
	require.JSONEq(t, rawOther, tokenLogs[0].Other)
}

func TestRecordChannelTestPerfMetricIncludesChannelTestSamples(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "10min",
		FlushInterval: 5,
	})
	insertEnabledChannel(t, 1, "default")

	start := time.Now().Add(-500 * time.Millisecond)
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName:   "channel-test-model",
		UsingGroup:        "default",
		StartTime:         start,
		FirstResponseTime: start.Add(100 * time.Millisecond),
		IsChannelTest:     true,
	}

	RecordRelayPerfMetric(nil, relayInfo, PerfMetricSample{Success: true})
	summary, err := GetPerfGroupHealthSummary(1, 10)
	require.NoError(t, err)
	defaultGroup := requirePerfGroupHealth(t, summary.Groups, "default")
	require.EqualValues(t, 0, defaultGroup.RequestCount)

	RecordChannelTestPerfMetric(nil, relayInfo, PerfMetricSample{
		Success:          true,
		CompletionTokens: 10,
	})
	summary, err = GetPerfGroupHealthSummary(1, 10)
	require.NoError(t, err)
	defaultGroup = requirePerfGroupHealth(t, summary.Groups, "default")
	require.EqualValues(t, 1, defaultGroup.RequestCount)
	require.Equal(t, 100.0, defaultGroup.SuccessRate)
	require.Equal(t, 0.0, defaultGroup.AvgLatencyMs)
	require.Equal(t, 0.0, defaultGroup.AvgTTFTMs)
	require.Equal(t, 0.0, defaultGroup.AvgTps)
	require.Zero(t, defaultGroup.RecentRequestCount)
	require.EqualValues(t, 0, defaultGroup.Buckets[len(defaultGroup.Buckets)-1].NonTestRequestCount)
}

func TestIgnoredAttemptsDoNotReachPerformanceBuckets(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{Enabled: true, BucketTime: "10min", FlushInterval: 5})
	insertEnabledChannel(t, 1, "default")
	info := &relaycommon.RelayInfo{OriginModelName: "test", UsingGroup: "default", StartTime: time.Now()}
	mapped := types.NewErrorWithStatusCode(errors.New("overloaded"), types.ErrorCodeBadResponse, 503)
	mapped.UpstreamStatusCode = 429
	for _, apiErr := range []*types.NewAPIError{
		mapped,
		types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponse, 429),
		types.NewErrorWithStatusCode(errors.New(upstreamHTTP2Failure), types.ErrorCodeBadResponse, 502),
		types.NewErrorWithStatusCode(errors.New(upstreamStreamFailure), types.ErrorCodeBadResponse, 502),
	} {
		sample := PerfMetricSample{RelayError: apiErr}
		RecordRelayPerfMetric(nil, info, sample)
		RecordChannelTestPerfMetric(nil, info, sample)
	}
	info.StreamStatus = relaycommon.NewStreamStatus()
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, nil)
	RecordRelayPerfMetric(nil, info, PerfMetricSample{Success: true, CompletionTokens: 50})
	info.StreamStatus = nil
	RecordRelayPerfMetric(nil, info, PerfMetricSample{Success: true})
	RecordRelayPerfMetric(nil, info, PerfMetricSample{RelayError: types.NewError(errors.New("server failure"), types.ErrorCodeBadResponse)})
	summary, err := GetPerfGroupHealthSummary(1, 10)
	require.NoError(t, err)
	group := requirePerfGroupHealth(t, summary.Groups, "default")
	require.EqualValues(t, 2, group.RequestCount)
	require.EqualValues(t, 2, group.RecentRequestCount)
	require.Equal(t, 50.0, group.SuccessRate)
	require.NoError(t, FlushPerfMetrics())
	var bucket PerfMetricBucket
	require.NoError(t, LOG_DB.Where("model_name = ?", "test").First(&bucket).Error)
	require.EqualValues(t, 2, bucket.RequestCount)
	require.EqualValues(t, 1, bucket.SuccessCount)
	require.Zero(t, bucket.TestRequestCount)
	require.Zero(t, bucket.CompletionTokens)
}

func TestRecordChannelTestPerfMetricFlushesToBuckets(t *testing.T) {
	truncateTables(t)
	ResetPerfMetricsForTest()
	common.SetPerfMetricsConfig(common.PerfMetricsConfig{
		Enabled:       true,
		BucketTime:    "10min",
		FlushInterval: 5,
	})
	insertEnabledChannel(t, 1, "default")

	start := time.Now().Add(-800 * time.Millisecond)
	RecordChannelTestPerfMetric(nil, &relaycommon.RelayInfo{
		OriginModelName:   "channel-test-flush-model",
		UsingGroup:        "default",
		StartTime:         start,
		FirstResponseTime: start.Add(120 * time.Millisecond),
		IsChannelTest:     true,
	}, PerfMetricSample{
		Success:          true,
		CompletionTokens: 12,
	})
	require.NoError(t, FlushPerfMetrics())

	ResetPerfMetricsForTest()
	summary, err := GetPerfGroupHealthSummary(1, 10)
	require.NoError(t, err)
	defaultGroup := requirePerfGroupHealth(t, summary.Groups, "default")
	require.EqualValues(t, 1, defaultGroup.RequestCount)
	require.Equal(t, 100.0, defaultGroup.SuccessRate)
	require.Equal(t, 0.0, defaultGroup.AvgLatencyMs)
	require.Equal(t, 0.0, defaultGroup.AvgTTFTMs)
	require.Equal(t, 0.0, defaultGroup.AvgTps)

	var bucket PerfMetricBucket
	require.NoError(t, LOG_DB.Where("model_name = ?", "channel-test-flush-model").First(&bucket).Error)
	require.EqualValues(t, 1, bucket.RequestCount)
	require.EqualValues(t, 1, bucket.SuccessCount)
	require.EqualValues(t, 1, bucket.TestRequestCount)
	require.Zero(t, bucket.LatencyCount)
	require.Zero(t, bucket.TotalLatencyMs)
	require.Zero(t, bucket.TTFTCount)
	require.Zero(t, bucket.CompletionTokens)
	require.Zero(t, bucket.TotalTPSLatencyMs)
}
