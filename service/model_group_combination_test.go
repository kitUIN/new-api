package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelGroupCombinationSettings(t *testing.T) {
	t.Helper()
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatio))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"group-a":"A","group-b":"B","group-c":"C","deprecated":"Deprecated"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"group-a":1,"group-b":2,"group-c":3}`))
}

func TestNormalizeTokenModelGroupCombination(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	token := &model.Token{
		Group:                        "auto",
		CrossGroupRetry:              true,
		AutoGroupMode:                RuleAutoGroupModeBalanced,
		ModelGroupCombinationEnabled: true,
		ModelGroupCombinationGroups:  `[" group-a ","group-b"]`,
	}

	require.NoError(t, NormalizeTokenModelGroupCombination(token, ""))
	require.Equal(t, "group-a", token.Group)
	require.False(t, token.CrossGroupRetry)
	require.Empty(t, token.AutoGroupMode)
	require.JSONEq(t, `["group-a","group-b"]`, token.ModelGroupCombinationGroups)
}

func TestNormalizeTokenModelGroupCombinationRejectsInvalidConfiguration(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	tests := []struct {
		name     string
		groups   string
		failover bool
	}{
		{name: "single group", groups: `["group-a"]`},
		{name: "duplicate group", groups: `["group-a","group-a"]`},
		{name: "auto group", groups: `["group-a","auto"]`},
		{name: "rule auto group", groups: `["group-a","自动分组:codex"]`},
		{name: "unauthorized group", groups: `["group-a","missing"]`},
		{name: "deprecated group", groups: `["group-a","deprecated"]`},
		{name: "invalid json", groups: `{`},
		{name: "failover enabled", groups: `["group-a","group-b"]`, failover: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &model.Token{
				ModelGroupCombinationEnabled: true,
				ModelGroupCombinationGroups:  tt.groups,
				SessionGroupFailoverEnabled:  tt.failover,
			}
			require.Error(t, NormalizeTokenModelGroupCombination(token, ""))
		})
	}
}

func TestResolveModelGroupCombinationUsesConfiguredOrderAndAvailability(t *testing.T) {
	setupModelGroupCombinationSettings(t)
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	require.NoError(t, db.Create([]model.Ability{
		{Group: "group-a", Model: "shared-model", ChannelId: 1, Enabled: true},
		{Group: "group-b", Model: "shared-model", ChannelId: 2, Enabled: true},
		{Group: "group-b", Model: "deepseek-v4-flash", ChannelId: 2, Enabled: true},
	}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationGroups, `["group-a","group-b"]`)

	group, enabled, err := ResolveModelGroupCombination(ctx, "shared-model")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "group-a", group)
	require.Equal(t, "group-a", common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))

	group, enabled, err = ResolveModelGroupCombination(ctx, "deepseek-v4-flash")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "group-b", group)

	_, enabled, err = ResolveModelGroupCombination(ctx, "missing-model")
	require.True(t, enabled)
	require.Error(t, err)
}
