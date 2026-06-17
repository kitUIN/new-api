package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestFormatUserLogsHidesErrorDetails(t *testing.T) {
	logs := []*Log{
		{
			Id:      99,
			Type:    LogTypeError,
			Content: "status_code=401, invalid API key: sk-secret",
			Other:   `{"status_code":401,"error_code":"invalid_api_key","error_type":"openai_error","channel_id":7,"admin_info":{"upstream_error_body":"secret upstream body"}}`,
		},
	}

	formatUserLogs(logs, 0)

	require.Equal(t, 1, logs[0].Id)
	require.Equal(t, "status_code=401", logs[0].Content)
	require.Contains(t, logs[0].Other, `"status_code":401`)
	require.Contains(t, logs[0].Other, `"error_code":"invalid_api_key"`)
	require.NotContains(t, logs[0].Other, "admin_info")
	require.NotContains(t, logs[0].Other, "upstream_error_body")
	require.NotContains(t, logs[0].Other, "channel_id")
}

func TestFormatUserLogsFallsBackToStatusCodeInContent(t *testing.T) {
	logs := []*Log{
		{
			Type:    LogTypeError,
			Content: "status_code=429, rate limit exceeded",
			Other:   "",
		},
	}

	formatUserLogs(logs, 0)

	require.Equal(t, "status_code=429", logs[0].Content)
	require.Equal(t, "{}", logs[0].Other)
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
}
