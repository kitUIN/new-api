package constant

const (
	RuleAutoGroupCodexLow = "auto:codex-low"
	RuleAutoGroupCodexPro = "auto:codex-pro"
	RuleAutoGroupKiro     = "auto:kiro"
	RuleAutoGroupGemini   = "auto:gemini"

	RuleAutoGroupModeLowRatio = "low_ratio"
	RuleAutoGroupModeBalanced = "balanced"
)

func IsRuleAutoGroup(group string) bool {
	switch group {
	case RuleAutoGroupCodexLow, RuleAutoGroupCodexPro, RuleAutoGroupKiro, RuleAutoGroupGemini:
		return true
	default:
		return false
	}
}
