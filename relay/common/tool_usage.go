package common

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var reservedBillableToolNames = map[string]struct{}{
	dto.BuildInToolWebSearchPreview: {},
	dto.BuildInToolWebSearch:        {},
	dto.BuildInToolFileSearch:       {},
	dto.BuildInToolGoogleSearch:     {},
	dto.BuildInToolImageGeneration:  {},
}

// CountBillableToolCall records built-in calls and configured custom function calls.
func (info *RelayInfo) CountBillableToolCall(itemType string, functionName string) {
	if info == nil {
		return
	}
	if info.ResponsesUsageInfo == nil {
		info.ResponsesUsageInfo = &ResponsesUsageInfo{BuiltInTools: make(map[string]*BuildInToolInfo)}
	}
	if info.ResponsesUsageInfo.BuiltInTools == nil {
		info.ResponsesUsageInfo.BuiltInTools = make(map[string]*BuildInToolInfo)
	}

	switch itemType {
	case dto.BuildInCallWebSearchCall:
		info.incrementBillableToolCall(resolveWebSearchToolName(info.ResponsesUsageInfo.BuiltInTools))
	case dto.BuildInCallFileSearchCall:
		info.incrementBillableToolCall(dto.BuildInToolFileSearch)
	case dto.BuildInCallFunctionCall, dto.BuildInCallToolUse:
		if functionName == "" {
			return
		}
		if _, reserved := reservedBillableToolNames[functionName]; reserved {
			return
		}
		if operation_setting.GetToolPriceForModel(functionName, info.OriginModelName) <= 0 {
			return
		}
		info.incrementBillableToolCall(functionName)
	}
}

func resolveWebSearchToolName(tools map[string]*BuildInToolInfo) string {
	if _, ok := tools[dto.BuildInToolWebSearchPreview]; ok {
		return dto.BuildInToolWebSearchPreview
	}
	if _, ok := tools[dto.BuildInToolWebSearch]; ok {
		return dto.BuildInToolWebSearch
	}
	return dto.BuildInToolWebSearchPreview
}

func (info *RelayInfo) incrementBillableToolCall(name string) {
	if existing, ok := info.ResponsesUsageInfo.BuiltInTools[name]; ok && existing != nil {
		existing.CallCount++
		return
	}
	info.ResponsesUsageInfo.BuiltInTools[name] = &BuildInToolInfo{
		ToolName:  name,
		CallCount: 1,
	}
}
