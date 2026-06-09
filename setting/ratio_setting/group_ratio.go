package ratio_setting

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

const (
	UpstreamGroupRatioBindingSourceChannel  = "channel"
	UpstreamGroupRatioBindingSourceProvider = "provider"
)

type UpstreamGroupRatioBinding struct {
	SourceType    string  `json:"source_type"`
	SourceID      int     `json:"source_id"`
	UpstreamGroup string  `json:"upstream_group"`
	Offset        float64 `json:"offset,omitempty"`
}

var upstreamGroupRatioBindingMap = types.NewRWMap[string, UpstreamGroupRatioBinding]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{
	"vip": {
		"append_1":   "vip_special_group_1",
		"-:remove_1": "vip_removed_group_1",
	},
}

type GroupRatioSetting struct {
	GroupRatio                 *types.RWMap[string, float64]                   `json:"group_ratio"`
	GroupGroupRatio            *types.RWMap[string, map[string]float64]        `json:"group_group_ratio"`
	GroupSpecialUsableGroup    *types.RWMap[string, map[string]string]         `json:"group_special_usable_group"`
	UpstreamGroupRatioBindings *types.RWMap[string, UpstreamGroupRatioBinding] `json:"upstream_group_ratio_bindings"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup:    groupSpecialUsableGroup,
		GroupRatio:                 groupRatioMap,
		GroupGroupRatio:            groupGroupRatioMap,
		UpstreamGroupRatioBindings: upstreamGroupRatioBindingMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	if groupRatioSetting.UpstreamGroupRatioBindings == nil {
		groupRatioSetting.UpstreamGroupRatioBindings = upstreamGroupRatioBindingMap
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func GetUpstreamGroupRatioBindingsCopy() map[string]UpstreamGroupRatioBinding {
	return upstreamGroupRatioBindingMap.ReadAll()
}

func UpstreamGroupRatioBindings2JSONString() string {
	return upstreamGroupRatioBindingMap.MarshalJSONString()
}

func UpdateUpstreamGroupRatioBindingsByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(upstreamGroupRatioBindingMap, jsonStr)
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := common.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}

func CheckUpstreamGroupRatioBindings(jsonStr string) error {
	bindings := make(map[string]UpstreamGroupRatioBinding)
	if err := common.Unmarshal([]byte(jsonStr), &bindings); err != nil {
		return err
	}
	for group, binding := range bindings {
		if strings.TrimSpace(group) == "" {
			return errors.New("bound group name cannot be empty")
		}
		switch binding.SourceType {
		case UpstreamGroupRatioBindingSourceChannel, UpstreamGroupRatioBindingSourceProvider:
		default:
			return errors.New("invalid upstream group ratio binding source type: " + group)
		}
		if binding.SourceID <= 0 {
			return errors.New("upstream group ratio binding source id must be greater than 0: " + group)
		}
		if strings.TrimSpace(binding.UpstreamGroup) == "" {
			return errors.New("upstream group ratio binding upstream group cannot be empty: " + group)
		}
		if math.IsNaN(binding.Offset) || math.IsInf(binding.Offset, 0) {
			return errors.New("upstream group ratio binding offset must be finite: " + group)
		}
	}
	return nil
}

func ApplyGroupRatioBindingLocksToJSONString(jsonStr string) (string, error) {
	nextGroupRatio := make(map[string]float64)
	if err := common.Unmarshal([]byte(jsonStr), &nextGroupRatio); err != nil {
		return "", err
	}

	currentGroupRatio := GetGroupRatioCopy()
	for group := range GetUpstreamGroupRatioBindingsCopy() {
		if ratio, ok := currentGroupRatio[group]; ok {
			nextGroupRatio[group] = ratio
		}
	}

	bytes, err := common.Marshal(nextGroupRatio)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
