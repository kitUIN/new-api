package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestApplyGroupRatioBindingLocksToJSONString(t *testing.T) {
	originalRatio := GroupRatio2JSONString()
	originalBindings := UpstreamGroupRatioBindings2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioByJSONString(originalRatio))
		require.NoError(t, UpdateUpstreamGroupRatioBindingsByJSONString(originalBindings))
	})

	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":1,"vip":0.8}`))
	require.NoError(t, UpdateUpstreamGroupRatioBindingsByJSONString(`{
		"vip": {
			"source_type": "channel",
			"source_id": 1,
			"upstream_group": "vip",
			"offset": 0.01
		}
	}`))

	locked, err := ApplyGroupRatioBindingLocksToJSONString(`{"default":2,"vip":9,"new":3}`)
	require.NoError(t, err)

	var got map[string]float64
	require.NoError(t, common.Unmarshal([]byte(locked), &got))
	require.Equal(t, 2.0, got["default"])
	require.Equal(t, 0.8, got["vip"])
	require.Equal(t, 3.0, got["new"])
}

func TestCheckUpstreamGroupRatioBindings(t *testing.T) {
	require.NoError(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "provider",
			"source_id": 2,
			"upstream_group": "default",
			"offset": -0.05
		}
	}`))

	require.Error(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "invalid",
			"source_id": 2,
			"upstream_group": "default"
		}
	}`))
	require.Error(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "channel",
			"source_id": 0,
			"upstream_group": "default"
		}
	}`))
	require.Error(t, CheckUpstreamGroupRatioBindings(`{
		"default": {
			"source_type": "channel",
			"source_id": 1,
			"upstream_group": ""
		}
	}`))
}

func TestCheckGroupRatioRejectsNegative(t *testing.T) {
	require.Error(t, CheckGroupRatio(`{"default":-0.1}`))
	require.NoError(t, CheckGroupRatio(`{"default":0}`))
}
