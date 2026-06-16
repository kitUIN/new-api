package setting

import (
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const disabledGroupDescriptionPrefix = "__disabled_description__:"

var userUsableGroups = map[string]string{
	"default": "默认分组",
	"vip":     "vip分组",
}
var userUsableGroupsMutex sync.RWMutex

func isDisabledGroupDescriptionKey(groupName string) bool {
	return strings.HasPrefix(groupName, disabledGroupDescriptionPrefix)
}

func GetUserUsableGroupsCopy() map[string]string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	copyUserUsableGroups := make(map[string]string)
	for k, v := range userUsableGroups {
		if isDisabledGroupDescriptionKey(k) {
			continue
		}
		copyUserUsableGroups[k] = v
	}
	return copyUserUsableGroups
}

func UserUsableGroups2JSONString() string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	jsonBytes, err := common.Marshal(userUsableGroups)
	if err != nil {
		common.SysLog("error marshalling user groups: " + err.Error())
	}
	return string(jsonBytes)
}

func BuildDisabledUserUsableGroupJSONString(groupName string) (string, bool, error) {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return "", false, nil
	}

	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	desc, ok := userUsableGroups[groupName]
	if !ok {
		return "", false, nil
	}

	nextGroups := make(map[string]string, len(userUsableGroups))
	for k, v := range userUsableGroups {
		nextGroups[k] = v
	}
	delete(nextGroups, groupName)
	nextGroups[disabledGroupDescriptionPrefix+groupName] = desc

	jsonBytes, err := common.Marshal(nextGroups)
	if err != nil {
		return "", false, err
	}
	return string(jsonBytes), true, nil
}

func UpdateUserUsableGroupsByJSONString(jsonStr string) error {
	userUsableGroupsMutex.Lock()
	defer userUsableGroupsMutex.Unlock()

	userUsableGroups = make(map[string]string)
	return common.UnmarshalJsonStr(jsonStr, &userUsableGroups)
}

func GetUsableGroupDescription(groupName string) string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	if desc, ok := userUsableGroups[groupName]; ok {
		return desc
	}
	return groupName
}
