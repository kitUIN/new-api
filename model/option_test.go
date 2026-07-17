package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
