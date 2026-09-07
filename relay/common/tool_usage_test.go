package common

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountBillableToolCallFunctionCallRequiresConfiguredPrice(t *testing.T) {
	operation_setting.SetToolPriceForTest("gemini_priced_fn", 5)
	t.Cleanup(func() {
		operation_setting.DeleteToolPriceForTest("gemini_priced_fn")
	})

	info := &RelayInfo{OriginModelName: "gemini-2.5-flash"}
	info.CountBillableToolCall(dto.BuildInCallFunctionCall, "gemini_priced_fn")
	require.NotNil(t, info.ResponsesUsageInfo)
	require.Contains(t, info.ResponsesUsageInfo.BuiltInTools, "gemini_priced_fn")
	assert.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools["gemini_priced_fn"].CallCount)

	info.CountBillableToolCall(dto.BuildInCallFunctionCall, "gemini_unpriced_fn")
	assert.NotContains(t, info.ResponsesUsageInfo.BuiltInTools, "gemini_unpriced_fn")
}

func TestCountBillableToolCallFunctionCallSkipsReservedNames(t *testing.T) {
	info := &RelayInfo{OriginModelName: "gemini-2.5-flash"}

	for _, name := range []string{
		dto.BuildInToolWebSearchPreview,
		dto.BuildInToolWebSearch,
		dto.BuildInToolFileSearch,
		dto.BuildInToolGoogleSearch,
		dto.BuildInToolImageGeneration,
	} {
		info.CountBillableToolCall(dto.BuildInCallFunctionCall, name)
	}

	if info.ResponsesUsageInfo != nil {
		assert.Empty(t, info.ResponsesUsageInfo.BuiltInTools)
	}
}
