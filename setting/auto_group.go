package setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	AutoGroupOrderTypePriority = "priority"
	AutoGroupOrderTypeRatioAsc = "ratio_asc"
)

var autoGroups = []string{
	"default",
}

var DefaultUseAutoGroup = false
var AutoGroupOrderType = AutoGroupOrderTypePriority

func ContainsAutoGroup(group string) bool {
	for _, autoGroup := range autoGroups {
		if autoGroup == group {
			return true
		}
	}
	return false
}

func UpdateAutoGroupsByJsonString(jsonString string) error {
	autoGroups = make([]string, 0)
	return common.Unmarshal([]byte(jsonString), &autoGroups)
}

func AutoGroups2JsonString() string {
	jsonBytes, err := common.Marshal(autoGroups)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func GetAutoGroups() []string {
	return autoGroups
}

func GetAutoGroupOrderType() string {
	orderType := strings.TrimSpace(AutoGroupOrderType)
	switch orderType {
	case AutoGroupOrderTypeRatioAsc:
		return AutoGroupOrderTypeRatioAsc
	default:
		return AutoGroupOrderTypePriority
	}
}

func UpdateAutoGroupOrderType(value string) {
	AutoGroupOrderType = normalizeAutoGroupOrderType(value)
}

func normalizeAutoGroupOrderType(value string) string {
	switch strings.TrimSpace(value) {
	case AutoGroupOrderTypeRatioAsc:
		return AutoGroupOrderTypeRatioAsc
	default:
		return AutoGroupOrderTypePriority
	}
}
