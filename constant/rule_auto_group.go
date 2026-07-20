package constant

const (
	RuleAutoGroupCodexLow = "自动分组:codex-low"
	RuleAutoGroupCodexPro = "自动分组:codex-pro"
	RuleAutoGroupKiro     = "自动分组:kiro"
	RuleAutoGroupGemini   = "自动分组:gemini"

	legacyRuleAutoGroupCodexLow = "auto:codex-low"
	legacyRuleAutoGroupCodexPro = "auto:codex-pro"
	legacyRuleAutoGroupKiro     = "auto:kiro"
	legacyRuleAutoGroupGemini   = "auto:gemini"

	RuleAutoGroupModeLowRatio = "low_ratio"
	RuleAutoGroupModeBalanced = "balanced"
)

func NormalizeRuleAutoGroupName(group string) string {
	switch group {
	case RuleAutoGroupCodexLow, legacyRuleAutoGroupCodexLow:
		return RuleAutoGroupCodexLow
	case RuleAutoGroupCodexPro, legacyRuleAutoGroupCodexPro:
		return RuleAutoGroupCodexPro
	case RuleAutoGroupKiro, legacyRuleAutoGroupKiro:
		return RuleAutoGroupKiro
	case RuleAutoGroupGemini, legacyRuleAutoGroupGemini:
		return RuleAutoGroupGemini
	default:
		return ""
	}
}

func IsRuleAutoGroup(group string) bool {
	return NormalizeRuleAutoGroupName(group) != ""
}
