package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionPersistsGroupTypes(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where("key = ?", "GroupTypes").Delete(&Option{}).Error)

	previousGroupTypes := ratio_setting.GroupTypes2JSONString()
	common.OptionMapRWMutex.Lock()
	optionMapWasNil := common.OptionMap == nil
	if optionMapWasNil {
		common.OptionMap = make(map[string]string)
	}
	previousOptionValue, hadPreviousOptionValue := common.OptionMap["GroupTypes"]
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupTypesByJSONString(previousGroupTypes))
		require.NoError(t, DB.Where("key = ?", "GroupTypes").Delete(&Option{}).Error)
		common.OptionMapRWMutex.Lock()
		if optionMapWasNil {
			common.OptionMap = nil
		} else if hadPreviousOptionValue {
			common.OptionMap["GroupTypes"] = previousOptionValue
		} else {
			delete(common.OptionMap, "GroupTypes")
		}
		common.OptionMapRWMutex.Unlock()
	})

	value := `{"vip":"user"}`
	require.NoError(t, UpdateOption("GroupTypes", value))

	var saved Option
	require.NoError(t, DB.First(&saved, "key = ?", "GroupTypes").Error)
	assert.JSONEq(t, value, saved.Value)
	assert.Equal(t, ratio_setting.GroupTypeUser, ratio_setting.GetGroupType("vip"))

	common.OptionMapRWMutex.RLock()
	assert.JSONEq(t, value, common.OptionMap["GroupTypes"])
	common.OptionMapRWMutex.RUnlock()
	assert.Equal(t, ratio_setting.GroupTypeBilling, ratio_setting.GetGroupType("missing"))
}

func TestHandleConfigUpdateRebuildsToolPriceIndex(t *testing.T) {
	cfg := config.GlobalConfig.Get("tool_price_setting")
	require.NotNil(t, cfg)
	before, err := config.ConfigToMap(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		handled, cleanupErr := handleConfigUpdate(operation_setting.ToolPriceOptionKey, before["prices"])
		require.True(t, handled)
		require.NoError(t, cleanupErr)
	})

	handled, err := handleConfigUpdate(operation_setting.ToolPriceOptionKey, `{"model_option_fn":6}`)
	require.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, 6.0, operation_setting.GetToolPriceForModel("model_option_fn", "gemini-2.5-flash"))
}
