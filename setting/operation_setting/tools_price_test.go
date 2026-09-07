package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveToolPrices(t *testing.T) {
	t.Helper()
	original := make(map[string]float64, len(toolPriceSetting.Prices))
	for name, price := range toolPriceSetting.Prices {
		original[name] = price
	}
	t.Cleanup(func() {
		toolPriceSetting.Prices = original
		RebuildToolPriceIndex()
	})
}

func TestToolPriceDefaultsAndLongestModelPrefix(t *testing.T) {
	preserveToolPrices(t)
	toolPriceSetting.Prices = map[string]float64{
		"custom_fn":                   3,
		"custom_fn:gemini-*":          5,
		"custom_fn:gemini-2.5-flash*": 7,
	}
	RebuildToolPriceIndex()

	assert.Equal(t, 14.0, GetGeminiGoogleSearchPricePerThousand())
	assert.Equal(t, 7.0, GetToolPriceForModel("custom_fn", "gemini-2.5-flash-preview"))
	assert.Equal(t, 5.0, GetToolPriceForModel("custom_fn", "gemini-3-pro"))
	assert.Equal(t, 3.0, GetToolPriceForModel("custom_fn", "other-model"))
}

func TestToolPriceExplicitZeroDisablesMatchingRule(t *testing.T) {
	preserveToolPrices(t)
	toolPriceSetting.Prices = map[string]float64{
		"custom_fn":                   4,
		"custom_fn:gemini-2.5-flash*": 0,
	}
	RebuildToolPriceIndex()

	assert.Equal(t, 0.0, GetToolPriceForModel("custom_fn", "gemini-2.5-flash"))
	assert.Equal(t, 4.0, GetToolPriceForModel("custom_fn", "gemini-2.5-pro"))
}

func TestValidateToolPricesJSON(t *testing.T) {
	for _, value := range []string{
		`{}`,
		`{"google_search":0}`,
		`{"custom_fn":2.5}`,
	} {
		require.NoError(t, ValidateToolPricesJSON(value), value)
	}

	for _, value := range []string{
		`null`,
		`[]`,
		`{"":1}`,
		`{"custom_fn":null}`,
		`{"custom_fn":"1"}`,
		`{"custom_fn":-1}`,
		`{"custom_fn":1e999}`,
	} {
		assert.Error(t, ValidateToolPricesJSON(value), value)
	}
}

func TestLoadToolPricesKeepsValidEntriesAndFallsBackForInvalidOnes(t *testing.T) {
	preserveToolPrices(t)

	LoadToolPricesFromJSONString(`{"custom_fn":3,"file_search":null,"google_search":-1}`)

	require.Len(t, toolPriceSetting.Prices, 1)
	assert.Equal(t, 3.0, GetToolPriceForModel("custom_fn", "gemini-2.5-flash"))
	assert.Equal(t, FileSearchPrice, GetFileSearchPricePerThousand())
	assert.Equal(t, GoogleSearchPrice, GetGeminiGoogleSearchPricePerThousand())
}
