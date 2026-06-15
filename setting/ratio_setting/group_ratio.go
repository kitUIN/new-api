package ratio_setting

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
	"github.com/expr-lang/expr"
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
	SourceType       string  `json:"source_type"`
	SourceID         int     `json:"source_id"`
	UpstreamGroup    string  `json:"upstream_group"`
	Offset           float64 `json:"offset,omitempty"`
	OffsetExpression string  `json:"-"`
}

func (binding *UpstreamGroupRatioBinding) UnmarshalJSON(data []byte) error {
	var payload struct {
		SourceType       string          `json:"source_type"`
		SourceID         int             `json:"source_id"`
		UpstreamGroup    string          `json:"upstream_group"`
		Offset           json.RawMessage `json:"offset"`
		OffsetExpression string          `json:"offset_expr"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return err
	}

	binding.SourceType = payload.SourceType
	binding.SourceID = payload.SourceID
	binding.UpstreamGroup = payload.UpstreamGroup
	binding.OffsetExpression = strings.TrimSpace(payload.OffsetExpression)
	binding.Offset = 0

	if len(payload.Offset) == 0 || common.GetJsonType(payload.Offset) == "null" {
		return nil
	}
	switch common.GetJsonType(payload.Offset) {
	case "string":
		var offsetExpression string
		if err := common.Unmarshal(payload.Offset, &offsetExpression); err != nil {
			return err
		}
		binding.OffsetExpression = strings.TrimSpace(offsetExpression)
	case "number":
		if err := common.Unmarshal(payload.Offset, &binding.Offset); err != nil {
			return err
		}
	default:
		return errors.New("upstream group ratio binding offset must be a number or expression string")
	}
	return nil
}

func (binding UpstreamGroupRatioBinding) MarshalJSON() ([]byte, error) {
	payload := map[string]interface{}{
		"source_type":    binding.SourceType,
		"source_id":      binding.SourceID,
		"upstream_group": binding.UpstreamGroup,
	}
	if expression := strings.TrimSpace(binding.OffsetExpression); expression != "" {
		payload["offset"] = expression
	} else if binding.Offset != 0 {
		payload["offset"] = binding.Offset
	}
	return common.Marshal(payload)
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

func CheckUpstreamGroupRatioOffsetExpression(expression string) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil
	}
	_, err := expr.Compile(
		expression,
		expr.Env(map[string]interface{}{"x": float64(0)}),
		expr.AsFloat64(),
	)
	if err != nil {
		return fmt.Errorf("invalid offset expression: %w", err)
	}
	return nil
}

func CalculateUpstreamGroupBoundRatio(upstreamRatio float64, binding UpstreamGroupRatioBinding) (float64, error) {
	if math.IsNaN(upstreamRatio) || math.IsInf(upstreamRatio, 0) {
		return 0, errors.New("upstream ratio must be finite")
	}

	var result float64
	if expression := strings.TrimSpace(binding.OffsetExpression); expression != "" {
		prog, err := expr.Compile(
			expression,
			expr.Env(map[string]interface{}{"x": float64(0)}),
			expr.AsFloat64(),
		)
		if err != nil {
			return 0, fmt.Errorf("offset expression compile error: %w", err)
		}
		output, err := expr.Run(prog, map[string]interface{}{"x": upstreamRatio})
		if err != nil {
			return 0, fmt.Errorf("offset expression run error: %w", err)
		}
		value, ok := output.(float64)
		if !ok {
			return 0, fmt.Errorf("offset expression result is %T, want float64", output)
		}
		result = value
	} else {
		if math.IsNaN(binding.Offset) || math.IsInf(binding.Offset, 0) {
			return 0, errors.New("offset must be finite")
		}
		result = upstreamRatio + binding.Offset
	}

	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0, errors.New("bound ratio must be finite")
	}
	if result < 0 {
		result = 0
	}
	return result, nil
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
		if err := CheckUpstreamGroupRatioOffsetExpression(binding.OffsetExpression); err != nil {
			return fmt.Errorf("upstream group ratio binding offset expression invalid: %s: %w", group, err)
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
