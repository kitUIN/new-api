package ratio_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGroupDrawingResolutions(t *testing.T) {
	original := groupRatioSetting.GroupDrawingResolutions
	t.Cleanup(func() { groupRatioSetting.GroupDrawingResolutions = original })
	config := &GroupDrawingResolutionsConfig{}
	groupRatioSetting.GroupDrawingResolutions = config
	require.NoError(t, common.Unmarshal([]byte(`{"A":["1K"],"B":["2K","4K"],"disabled":[]}`), config))
	require.True(t, GroupSupportsDrawingResolution("A", "1K"))
	require.False(t, GroupSupportsDrawingResolution("A", "2K"))
	require.True(t, GroupSupportsDrawingResolution("B", "2K"))
	require.True(t, GroupSupportsDrawingResolution("B", "4K"))
	require.True(t, GroupSupportsDrawingResolution("legacy", "4K"))
	require.False(t, GroupSupportsDrawingResolution("disabled", "1K"))
	for _, value := range []string{`null`, `[]`, `{"A":null}`, `{"A":["8K"]}`, `{"A":["1K","1K"]}`, `{" A ":["1K"]}`} {
		require.Error(t, CheckGroupDrawingResolutions(value), value)
	}
	data, err := common.Marshal(config)
	require.NoError(t, err)
	require.JSONEq(t, `{"A":["1K"],"B":["2K","4K"],"disabled":[]}`, string(data))
}

func TestDrawingResolutionForSize(t *testing.T) {
	for size, expected := range map[string]string{"": "1K", "auto": "1K", "1024x1792": "1K", "1792x1024": "1K", "2048x2736": "2K", "3584x2048": "2K", "4096x7168": "4K", "3008x2400": "4K"} {
		actual, err := DrawingResolutionForSize(size)
		require.NoError(t, err)
		require.Equal(t, expected, actual, size)
	}
	for _, size := range []string{"0x1024", "-1x2000", "2K", "2048x", "1x2x3", "8192x8192"} {
		_, err := DrawingResolutionForSize(size)
		require.Error(t, err, size)
	}
}

func TestDrawingResolutionRejectsUnalignedSizes(t *testing.T) {
	for _, size := range []string{"1025x1024", "1024x1025", "1024x1365", "2048x2731", "4096x5461"} {
		_, err := DrawingResolutionForSize(size)
		require.ErrorContains(t, err, "图片宽度和高度必须为 16 的倍数", size)
	}
}
