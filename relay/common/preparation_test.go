package common

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRemoveDisabledFieldsPreservesUnchangedJSON(t *testing.T) {
	input := []byte("{\n  \"input\": [{\"id\":9007199254740993,\"zero\":0,\"flag\":false}], \"store\": false\n}")
	out, err := RemoveDisabledFields(input, dto.ChannelOtherSettings{}, false)
	require.NoError(t, err)
	require.Equal(t, string(input), string(out))
	require.True(t, &input[0] == &out[0], "no-op filtering should reuse the original bytes")
}

func TestRemoveDisabledFieldsPreservesRawValuesWhenChanged(t *testing.T) {
	input := []byte(`{"service_tier":"priority","safety_identifier":"private","store":false,"stream_options":{"include_obfuscation":false,"include_usage":false},"input":{"id":9007199254740993,"zero":0,"exponent":1e+09,"flag":false}}`)
	original := string(input)
	out, err := RemoveDisabledFields(input, dto.ChannelOtherSettings{}, false)
	require.NoError(t, err)
	require.Equal(t, original, string(input))
	require.False(t, gjson.GetBytes(out, "service_tier").Exists())
	require.False(t, gjson.GetBytes(out, "safety_identifier").Exists())
	require.False(t, gjson.GetBytes(out, "stream_options.include_obfuscation").Exists())
	require.Equal(t, "false", gjson.GetBytes(out, "stream_options.include_usage").Raw)
	require.Equal(t, "false", gjson.GetBytes(out, "store").Raw)
	require.Equal(t, gjson.GetBytes(input, "input").Raw, gjson.GetBytes(out, "input").Raw)
	for _, input := range []string{`{"input":`, `[]`, `"text"`} {
		out, err := RemoveDisabledFields([]byte(input), dto.ChannelOtherSettings{}, false)
		require.NoError(t, err)
		require.Equal(t, input, string(out))
	}
}

func TestPreparationTimingAccumulatesAcrossRetriesAndReturnsSnapshot(t *testing.T) {
	info := &RelayInfo{}
	info.RecordPreparationDuration("responses_copy", 1250*time.Microsecond)
	info.MarkUpstreamRequestStart()
	info.RecordPreparationDuration("responses_copy", 250*time.Microsecond)
	info.RecordPreparationDuration("token_estimate", 5*time.Millisecond)
	info.RecordPreparationDuration("invalid", -time.Second)
	snapshot := info.PreparationTimingMs()
	require.Equal(t, map[string]float64{"responses_copy": 1.5, "token_estimate": 5}, snapshot)
	snapshot["responses_copy"] = 100
	require.Equal(t, 1.5, info.PreparationTimingMs()["responses_copy"])
}
