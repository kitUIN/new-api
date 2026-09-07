package operation_setting

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const ToolPriceOptionKey = "tool_price_setting.prices"

const (
	// Web search
	WebSearchPriceHigh = 25.00
	WebSearchPrice     = 10.00
	GoogleSearchPrice  = 14.00
	// File search
	FileSearchPrice = 2.5
)

type ToolPriceSetting struct {
	Prices map[string]float64 `json:"prices"`
}

type toolPricePrefixEntry struct {
	prefix string
	price  float64
}

type toolPriceIndex struct {
	defaults map[string]float64
	prefixes map[string][]toolPricePrefixEntry
}

var toolPriceSetting = ToolPriceSetting{Prices: make(map[string]float64)}
var currentToolPriceIndex atomic.Pointer[toolPriceIndex]

func init() {
	config.GlobalConfig.Register("tool_price_setting", &toolPriceSetting)
	RebuildToolPriceIndex()
}

func seedDefaultToolPrices(prices map[string]float64) {
	prices["web_search"] = ClaudeWebSearchPrice
	prices["web_search_preview"] = WebSearchPrice
	prices["file_search"] = FileSearchPrice
	prices["google_search"] = GoogleSearchPrice
	prices["web_search_preview:gpt-4o*"] = WebSearchPriceHigh
	prices["web_search_preview:gpt-4.1*"] = WebSearchPriceHigh
	prices["web_search_preview:gpt-4o-mini*"] = WebSearchPriceHigh
	prices["web_search_preview:gpt-4.1-mini*"] = WebSearchPriceHigh
}

func validToolPrice(price float64) bool {
	return price >= 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

func decodeToolPricesJSON(value string, ignoreInvalidEntries bool) (map[string]float64, error) {
	rawValue := json.RawMessage(strings.TrimSpace(value))
	if common.GetJsonType(rawValue) != "object" {
		return nil, fmt.Errorf("工具价格必须是 JSON 对象")
	}

	var rawPrices map[string]json.RawMessage
	if err := common.Unmarshal(rawValue, &rawPrices); err != nil {
		return nil, fmt.Errorf("解析工具价格失败: %w", err)
	}
	prices := make(map[string]float64, len(rawPrices))
	for name, rawPrice := range rawPrices {
		var entryErr error
		if strings.TrimSpace(name) == "" {
			entryErr = fmt.Errorf("工具名称不能为空")
		} else if common.GetJsonType(rawPrice) != "number" {
			entryErr = fmt.Errorf("工具价格 %q 必须是非负数字", name)
		} else {
			var price float64
			if err := common.Unmarshal(rawPrice, &price); err != nil {
				entryErr = fmt.Errorf("解析工具价格 %q 失败: %w", name, err)
			} else if !validToolPrice(price) {
				entryErr = fmt.Errorf("工具价格 %q 必须是有限的非负数字", name)
			} else {
				prices[name] = price
			}
		}
		if entryErr == nil {
			continue
		}
		if !ignoreInvalidEntries {
			return nil, entryErr
		}
		common.SysError(entryErr.Error())
	}
	return prices, nil
}

func ValidateToolPricesJSON(value string) error {
	_, err := decodeToolPricesJSON(value, false)
	return err
}

func LoadToolPricesFromJSONString(value string) {
	prices, err := decodeToolPricesJSON(value, true)
	if err != nil {
		common.SysError("加载工具价格失败，将使用硬编码兜底: " + err.Error())
		prices = make(map[string]float64)
	}
	toolPriceSetting.Prices = prices
	RebuildToolPriceIndex()
}

func RebuildToolPriceIndex() {
	merged := make(map[string]float64, 8+len(toolPriceSetting.Prices))
	seedDefaultToolPrices(merged)
	for name, price := range toolPriceSetting.Prices {
		if validToolPrice(price) {
			merged[name] = price
		}
	}

	index := &toolPriceIndex{
		defaults: make(map[string]float64),
		prefixes: make(map[string][]toolPricePrefixEntry),
	}
	for key, price := range merged {
		colon := strings.IndexByte(key, ':')
		if colon < 0 {
			index.defaults[key] = price
			continue
		}
		toolName := key[:colon]
		prefix := strings.TrimSuffix(key[colon+1:], "*")
		index.prefixes[toolName] = append(index.prefixes[toolName], toolPricePrefixEntry{prefix: prefix, price: price})
	}
	for toolName, entries := range index.prefixes {
		sort.Slice(entries, func(i, j int) bool {
			if len(entries[i].prefix) == len(entries[j].prefix) {
				return entries[i].prefix < entries[j].prefix
			}
			return len(entries[i].prefix) > len(entries[j].prefix)
		})
		index.prefixes[toolName] = entries
	}
	currentToolPriceIndex.Store(index)
}

func GetToolPriceForModel(toolName, modelName string) float64 {
	index := currentToolPriceIndex.Load()
	if index == nil {
		RebuildToolPriceIndex()
		index = currentToolPriceIndex.Load()
		if index == nil {
			return 0
		}
	}
	if entries, ok := index.prefixes[toolName]; ok && modelName != "" {
		for _, entry := range entries {
			if strings.HasPrefix(modelName, entry.prefix) {
				return entry.price
			}
		}
	}
	return index.defaults[toolName]
}

func GetToolPrice(toolName string) float64 {
	return GetToolPriceForModel(toolName, "")
}

func SetToolPriceForTest(name string, price float64) {
	if toolPriceSetting.Prices == nil {
		toolPriceSetting.Prices = make(map[string]float64)
	}
	toolPriceSetting.Prices[name] = price
	RebuildToolPriceIndex()
}

func DeleteToolPriceForTest(name string) {
	delete(toolPriceSetting.Prices, name)
	RebuildToolPriceIndex()
}

const (
	GPTImage1Low1024x1024    = 0.011
	GPTImage1Low1024x1536    = 0.016
	GPTImage1Low1536x1024    = 0.016
	GPTImage1Medium1024x1024 = 0.042
	GPTImage1Medium1024x1536 = 0.063
	GPTImage1Medium1536x1024 = 0.063
	GPTImage1High1024x1024   = 0.167
	GPTImage1High1024x1536   = 0.25
	GPTImage1High1536x1024   = 0.25
)

const (
	// Gemini Audio Input Price
	Gemini25FlashPreviewInputAudioPrice     = 1.00
	Gemini25FlashProductionInputAudioPrice  = 1.00 // for `gemini-2.5-flash`
	Gemini25FlashLitePreviewInputAudioPrice = 0.50
	Gemini25FlashNativeAudioInputAudioPrice = 3.00
	Gemini20FlashInputAudioPrice            = 0.70
	GeminiRoboticsER15InputAudioPrice       = 1.00
)

const (
	// Claude Web search
	ClaudeWebSearchPrice = 10.00
)

func GetClaudeWebSearchPricePerThousand() float64 {
	return GetToolPriceForModel("web_search", "")
}

func GetWebSearchPricePerThousand(modelName string, contextSize string) float64 {
	return GetToolPriceForModel("web_search_preview", modelName)
}

func GetGeminiGoogleSearchPricePerThousand() float64 {
	return GetToolPriceForModel("google_search", "")
}

func GetFileSearchPricePerThousand() float64 {
	return GetToolPriceForModel("file_search", "")
}

func GetGeminiInputAudioPricePerMillionTokens(modelName string) float64 {
	if strings.HasPrefix(modelName, "gemini-2.5-flash-preview-native-audio") {
		return Gemini25FlashNativeAudioInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-2.5-flash-preview-lite") {
		return Gemini25FlashLitePreviewInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-2.5-flash-preview") {
		return Gemini25FlashPreviewInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-2.5-flash") {
		return Gemini25FlashProductionInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-2.0-flash") {
		return Gemini20FlashInputAudioPrice
	} else if strings.HasPrefix(modelName, "gemini-robotics-er-1.5") {
		return GeminiRoboticsER15InputAudioPrice
	}
	return 0
}

func GetGPTImage1PriceOnceCall(quality string, size string) float64 {
	prices := map[string]map[string]float64{
		"low": {
			"1024x1024": GPTImage1Low1024x1024,
			"1024x1536": GPTImage1Low1024x1536,
			"1536x1024": GPTImage1Low1536x1024,
		},
		"medium": {
			"1024x1024": GPTImage1Medium1024x1024,
			"1024x1536": GPTImage1Medium1024x1536,
			"1536x1024": GPTImage1Medium1536x1024,
		},
		"high": {
			"1024x1024": GPTImage1High1024x1024,
			"1024x1536": GPTImage1High1024x1536,
			"1536x1024": GPTImage1High1536x1024,
		},
	}

	if qualityMap, exists := prices[quality]; exists {
		if price, exists := qualityMap[size]; exists {
			return price
		}
	}

	return GPTImage1High1024x1024
}
