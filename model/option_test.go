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
	const optionKey = "LogRequestDetailMode"
	const legacyKey = "LogRequestDetailEnabled"
	const requestId = "request-detail-option-test"
	previousMode := common.LogRequestDetailMode.Swap(common.RequestDetailLogOff)
	require.NoError(t, LOG_DB.AutoMigrate(&RequestDetail{}))

	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.LogRequestDetailMode.Store(previousMode)
		require.NoError(t, DB.Where("key IN ?", []string{optionKey, legacyKey}).Delete(&Option{}).Error)
		require.NoError(t, LOG_DB.Where("request_id = ?", requestId).Delete(&RequestDetail{}).Error)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
	})

	InitOptionMap()
	require.Equal(t, "0", common.OptionMap[optionKey])

	var expectedCount int64
	for _, mode := range []int32{common.RequestDetailLogOff, common.RequestDetailLogFailed, common.RequestDetailLogAll, common.RequestDetailLogOff} {
		value := strconv.Itoa(int(mode))
		require.NoError(t, UpdateOption(optionKey, value))
		var saved Option
		require.NoError(t, DB.First(&saved, "key = ?", optionKey).Error)
		require.Equal(t, value, saved.Value)
		common.LogRequestDetailMode.Store(-1)
		loadOptionsFromDatabase()
		require.Equal(t, mode, common.LogRequestDetailMode.Load())
		require.Equal(t, value, common.OptionMap[optionKey])

		for _, failed := range []bool{false, true} {
			RecordRequestDetail(requestId, 1, "{}", "request body", "{}", "response body", failed)
			if mode == common.RequestDetailLogAll || (mode == common.RequestDetailLogFailed && failed) {
				expectedCount++
			}
			var count int64
			require.NoError(t, LOG_DB.Model(&RequestDetail{}).Where("request_id = ?", requestId).Count(&count).Error)
			require.Equal(t, expectedCount, count)
		}
	}

	for _, invalid := range []string{"-1", "3", "true", "", "1.5"} {
		require.Error(t, UpdateOption(optionKey, invalid))
		require.Equal(t, common.RequestDetailLogOff, common.LogRequestDetailMode.Load())
		var saved Option
		require.NoError(t, DB.First(&saved, "key = ?", optionKey).Error)
		require.Equal(t, "0", saved.Value)
		require.Equal(t, "0", common.OptionMap[optionKey])
	}

	// Upgrade from the boolean option, then ensure a saved mode takes precedence
	// on every database sync, including when the chosen mode is off.
	require.NoError(t, DB.Where("key = ?", optionKey).Delete(&Option{}).Error)
	for _, enabled := range []bool{true, false} {
		require.NoError(t, DB.Save(&Option{Key: legacyKey, Value: strconv.FormatBool(enabled)}).Error)
		InitOptionMap()
		expectedMode := common.RequestDetailLogOff
		if enabled {
			expectedMode = common.RequestDetailLogFailed
		}
		require.Equal(t, expectedMode, common.LogRequestDetailMode.Load())
		require.Equal(t, strconv.Itoa(int(expectedMode)), common.OptionMap[optionKey])
	}
	require.NoError(t, DB.Save(&Option{Key: legacyKey, Value: "true"}).Error)
	for _, value := range []string{"2", "0"} {
		require.NoError(t, UpdateOption(optionKey, value))
		loadOptionsFromDatabase()
		require.Equal(t, value, common.OptionMap[optionKey])
		require.Equal(t, value, strconv.Itoa(int(common.LogRequestDetailMode.Load())))
	}
	// Old clients can still submit the boolean, which now saves the canonical mode.
	require.NoError(t, UpdateOption(legacyKey, "true"))
	loadOptionsFromDatabase()
	require.Equal(t, common.RequestDetailLogFailed, common.LogRequestDetailMode.Load())
	require.NoError(t, UpdateOption(legacyKey, "false"))
	loadOptionsFromDatabase()
	require.Equal(t, common.RequestDetailLogOff, common.LogRequestDetailMode.Load())

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
