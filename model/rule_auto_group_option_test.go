package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func TestRuleAutoGroupsOptionPersistence(t *testing.T) {
	setupBillingCostTest(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	originalEnabled := setting.RuleAutoGroupsEnabled()
	originalOptions := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		setting.SetRuleAutoGroupsEnabled(originalEnabled)
		common.OptionMap = originalOptions
	})
	for _, value := range []string{"false", "true"} {
		require.NoError(t, UpdateOption("RuleAutoGroupsEnabled", value))
		require.Equal(t, value == "true", setting.RuleAutoGroupsEnabled())
		var option Option
		require.NoError(t, DB.Where(&Option{Key: "RuleAutoGroupsEnabled"}).First(&option).Error)
		require.Equal(t, value, option.Value)
		// Loading a persisted option restores the runtime switch as well.
		setting.SetRuleAutoGroupsEnabled(value != "true")
		require.NoError(t, updateOptionMap(option.Key, option.Value))
		require.Equal(t, value == "true", setting.RuleAutoGroupsEnabled())
	}
}
