package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
