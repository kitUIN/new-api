package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRuleAutoGroupsEnableDisable(t *testing.T) {
	originalEnabled := setting.RuleAutoGroupsEnabled()
	originalDB := model.DB
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		setting.SetRuleAutoGroupsEnabled(originalEnabled)
		model.DB = originalDB
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatio))
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	require.NoError(t, db.Create(&model.Channel{Key: "test", Group: "gemini-a", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gemini-a":"Gemini","auto":"Auto"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gemini-a":0.2}`))
	for _, enabled := range []bool{true, false, true} {
		setting.SetRuleAutoGroupsEnabled(enabled)
		groups, err := GetSortedUserUsableGroupInfos("")
		require.NoError(t, err)
		available := UserUsableGroupInfosToMap(groups)
		require.Contains(t, available, "gemini-a")
		require.Contains(t, available, "auto")
		_, found := available[RuleAutoGroupGemini]
		require.Equal(t, enabled, found)
		for _, selector := range []string{RuleAutoGroupGemini, "auto:gemini"} {
			require.Equal(t, enabled, GroupInUserUsableGroups("", selector))
			err = NormalizeTokenRuleAutoGroup(&model.Token{Group: selector}, "")
			if enabled {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Empty(t, GetRuleAutoGroupCandidatesForModel("", selector, "test-model"))
			}
		}
	}
}

func TestRuleAutoGroupCandidatesUseEffectiveRatioAndBoundaries(t *testing.T) {
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatio := ratio_setting.GroupRatio2JSONString()
	originalOverrides := ratio_setting.GroupGroupRatio2JSONString()
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalOverrides))
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"codex-a":"A","codex-pro-a":"Pro A","codex-b":"B","codex-c":"C","codex-combo":"Combo"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"codex-a":0.05,"codex-pro-a":0.08,"codex-b":0.1,"codex-c":0.2,"codex-combo":0.01}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"codex-c":0.03}}`))
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(`{"codex-combo":[{"group":"codex-a","models":["test"]},{"group":"codex-b","models":["test"]}]}`))

	require.Equal(t, []string{"codex-a", "codex-pro-a"}, GetRuleAutoGroupCandidates("", RuleAutoGroupCodexLow))
	require.Equal(t, []string{"codex-a", "codex-pro-a"}, GetRuleAutoGroupCandidates("", "auto:codex-low"))
	require.Equal(t, []string{"codex-a", "codex-pro-a", "codex-b", "codex-c"}, GetRuleAutoGroupCandidates("", RuleAutoGroupCodex))
	require.True(t, GroupInUserUsableGroups("", RuleAutoGroupCodexLow))
	require.True(t, GroupInUserUsableGroups("", RuleAutoGroupCodex))
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

	token := &model.Token{Group: "auto:gemini", CrossGroupRetry: true}
	require.NoError(t, NormalizeTokenRuleAutoGroup(token, ""))
	require.Equal(t, RuleAutoGroupGemini, token.Group)
	require.Equal(t, RuleAutoGroupModeLowRatio, token.AutoGroupMode)
	require.False(t, token.CrossGroupRetry)

	token.AutoGroupMode = RuleAutoGroupModeBalanced
	require.NoError(t, NormalizeTokenRuleAutoGroup(token, ""))
	require.Equal(t, RuleAutoGroupModeBalanced, token.AutoGroupMode)

	token.SessionGroupFailoverEnabled = true
	require.Error(t, NormalizeTokenRuleAutoGroup(token, ""))
}

func TestRuleAutoGroupRatioRangeDisplay(t *testing.T) {
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatio))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gemini-a":"A","gemini-b":"B"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gemini-a":0.25,"gemini-b":0.3}`))

	infos := GetRuleAutoGroupInfosForUser("", map[string]bool{"gemini-a": true, "gemini-b": true})
	for _, info := range infos {
		if info.Name != RuleAutoGroupGemini {
			continue
		}
		require.NotNil(t, info.RatioRange)
		require.Equal(t, "0.25x~0.3x", info.RatioRange.Display)
		return
	}
	t.Fatal("gemini rule auto group not found")
}

func TestRuleAutoGroupAdminInfosOnlyIncludeEnabledGroups(t *testing.T) {
	originalRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatio))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"codex-enabled":0.05,"codex-disabled":0.2,"gemini-disabled":0.3}`))

	infos := GetRuleAutoGroupAdminInfos(map[string]bool{"codex-enabled": true})
	require.NotEmpty(t, infos)
	for _, info := range infos {
		require.Equal(t, []string{"codex-enabled"}, info.MatchedGroups)
		require.NotEqual(t, RuleAutoGroupGemini, info.Name)
	}
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
