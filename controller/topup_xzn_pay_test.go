package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetXznPayCallbackAddress(t *testing.T) {
	originalXznPayCallbackAddress := setting.XznPayCallbackAddress
	originalCustomCallbackAddress := operation_setting.CustomCallbackAddress
	originalServerAddress := system_setting.ServerAddress
	t.Cleanup(func() {
		setting.XznPayCallbackAddress = originalXznPayCallbackAddress
		operation_setting.CustomCallbackAddress = originalCustomCallbackAddress
		system_setting.ServerAddress = originalServerAddress
	})

	setting.XznPayCallbackAddress = " https://xzn.example.com/// "
	require.Equal(t, "https://xzn.example.com", getXznPayCallbackAddress())

	setting.XznPayCallbackAddress = ""
	operation_setting.CustomCallbackAddress = "https://payments.example.com/"
	require.Equal(t, "https://payments.example.com", getXznPayCallbackAddress())

	operation_setting.CustomCallbackAddress = ""
	system_setting.ServerAddress = "https://server.example.com/"
	require.Equal(t, "https://server.example.com", getXznPayCallbackAddress())
}

func TestParseXznPayMoneyCents(t *testing.T) {
	testCases := []struct {
		value    string
		expected int64
		valid    bool
	}{
		{value: "0.01", expected: 1, valid: true},
		{value: "10", expected: 1000, valid: true},
		{value: "10.50", expected: 1050, valid: true},
		{value: "10.001", valid: false},
		{value: "-1.00", valid: false},
		{value: "invalid", valid: false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.value, func(t *testing.T) {
			actual, err := parseXznPayMoneyCents(testCase.value)
			if testCase.valid {
				require.NoError(t, err)
				assert.Equal(t, testCase.expected, actual)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestBuildXznPayEventKeyIsStableAndStatusSensitive(t *testing.T) {
	params := map[string]string{
		"trade_no":       "P1",
		"out_trade_no":   "ORDER1",
		"transaction_id": "TX1",
		"refund_amount":  "5.00",
	}
	first := buildXznPayEventKey(params, "TRADE_REFUND", 123)
	second := buildXznPayEventKey(params, "TRADE_REFUND", 123)
	different := buildXznPayEventKey(params, "TRADE_FREEZE", 123)
	assert.Equal(t, first, second)
	assert.NotEqual(t, first, different)
}
