package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogRequestDetailOptionControlsRecording(t *testing.T) {
	const optionKey = "LogRequestDetailEnabled"
	const requestId = "request-detail-option-test"
	require.False(t, common.LogRequestDetailEnabled.Load())
	require.NoError(t, LOG_DB.AutoMigrate(&RequestDetail{}))

	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.LogRequestDetailEnabled.Store(false)
		require.NoError(t, DB.Where("key = ?", optionKey).Delete(&Option{}).Error)
		require.NoError(t, LOG_DB.Where("request_id = ?", requestId).Delete(&RequestDetail{}).Error)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
	})

	InitOptionMap()
	common.OptionMapRWMutex.RLock()
	defaultValue := common.OptionMap[optionKey]
	common.OptionMapRWMutex.RUnlock()
	require.Equal(t, "false", defaultValue)

	recordAndCount := func() int64 {
		RecordRequestDetail(requestId, 1, "{}", "request body", "{}", "response body")
		var count int64
		require.NoError(t, LOG_DB.Model(&RequestDetail{}).Where("request_id = ?", requestId).Count(&count).Error)
		return count
	}
	require.Zero(t, recordAndCount())

	for _, enabled := range []bool{true, false} {
		value := strconv.FormatBool(enabled)
		require.NoError(t, UpdateOption(optionKey, value))
		require.Equal(t, enabled, common.LogRequestDetailEnabled.Load())
		var saved Option
		require.NoError(t, DB.First(&saved, "key = ?", optionKey).Error)
		require.Equal(t, value, saved.Value)

		common.LogRequestDetailEnabled.Store(!enabled)
		loadOptionsFromDatabase()
		require.Equal(t, enabled, common.LogRequestDetailEnabled.Load())
		require.EqualValues(t, 1, recordAndCount())
	}

	var detail RequestDetail
	require.NoError(t, LOG_DB.First(&detail, "request_id = ?", requestId).Error)
	require.Equal(t, "request body", detail.RequestBody)
	require.Equal(t, "response body", detail.ResponseBody)
}

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
