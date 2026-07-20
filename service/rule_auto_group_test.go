package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestRuleAutoGroupCandidatesUseEffectiveRatioAndBoundaries(t *testing.T) {
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatio := ratio_setting.GroupRatio2JSONString()
	originalOverrides := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalOverrides))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"codex-a":"A","codex-pro-a":"Pro A","codex-b":"B","codex-c":"C"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"codex-a":0.05,"codex-pro-a":0.08,"codex-b":0.1,"codex-c":0.2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"codex-c":0.03}}`))

	require.Equal(t, []string{"codex-a", "codex-pro-a"}, GetRuleAutoGroupCandidates("", RuleAutoGroupCodexLow))
	require.Equal(t, []string{"codex-pro-a"}, GetRuleAutoGroupCandidates("", RuleAutoGroupCodexPro))
	require.Equal(t, []string{"codex-c", "codex-a", "codex-pro-a"}, GetRuleAutoGroupCandidates("vip", RuleAutoGroupCodexLow))
}

func TestNormalizeTokenRuleAutoGroup(t *testing.T) {
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatio))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gemini-a":"Gemini"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gemini-a":0.2}`))

	token := &model.Token{Group: RuleAutoGroupGemini, CrossGroupRetry: true}
	require.NoError(t, NormalizeTokenRuleAutoGroup(token, ""))
	require.Equal(t, RuleAutoGroupModeLowRatio, token.AutoGroupMode)
	require.False(t, token.CrossGroupRetry)

	token.AutoGroupMode = RuleAutoGroupModeBalanced
	require.NoError(t, NormalizeTokenRuleAutoGroup(token, ""))
	require.Equal(t, RuleAutoGroupModeBalanced, token.AutoGroupMode)

	token.SessionGroupFailoverEnabled = true
	require.Error(t, NormalizeTokenRuleAutoGroup(token, ""))
}

func TestNextRuleAutoGroupState(t *testing.T) {
	state := RuleAutoGroupState{Candidates: []string{"a", "b", "c"}, CandidateIndex: 0}

	state, switched, reason := nextRuleAutoGroupState(state, RuleAutoGroupModeLowRatio, false, true, 0, false)
	require.False(t, switched)
	require.Empty(t, reason)
	require.Equal(t, 1, state.FailureCount)

	state, switched, reason = nextRuleAutoGroupState(state, RuleAutoGroupModeLowRatio, false, true, 0, false)
	require.True(t, switched)
	require.Equal(t, "consecutive_failures", reason)
	require.Equal(t, 1, state.CandidateIndex)
	require.Zero(t, state.FailureCount)

	state, switched, _ = nextRuleAutoGroupState(state, RuleAutoGroupModeBalanced, true, false, 10_001, true)
	require.False(t, switched)
	require.Equal(t, 1, state.SlowTTFTCount)
	state, switched, _ = nextRuleAutoGroupState(state, RuleAutoGroupModeBalanced, true, false, 10_000, true)
	require.False(t, switched)
	require.Zero(t, state.SlowTTFTCount)
	state, switched, _ = nextRuleAutoGroupState(state, RuleAutoGroupModeBalanced, true, false, 10_001, true)
	require.False(t, switched)
	state, switched, reason = nextRuleAutoGroupState(state, RuleAutoGroupModeBalanced, true, false, 10_001, true)
	require.True(t, switched)
	require.Equal(t, "slow_ttft", reason)
	require.Equal(t, 2, state.CandidateIndex)

	state, switched, _ = nextRuleAutoGroupState(state, RuleAutoGroupModeBalanced, false, true, 0, false)
	require.False(t, switched)
	state, switched, _ = nextRuleAutoGroupState(state, RuleAutoGroupModeBalanced, false, true, 0, false)
	require.False(t, switched)
	require.Equal(t, 2, state.CandidateIndex)
}
