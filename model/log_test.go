package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

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
