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

func TestNormalizeTokenSessionFailoverRejectsInvalidGroups(t *testing.T) {
	tests := []struct {
		name   string
		groups string
	}{
		{
			name:   "auto group",
			groups: `["default","auto"]`,
		},
		{
			name:   "duplicate group",
			groups: `["default","default"]`,
		},
		{
			name:   "single group",
			groups: `["default"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &model.Token{
				SessionGroupFailoverEnabled: true,
				SessionFailoverGroups:       tt.groups,
				SessionFailoverThreshold:    3,
			}

			require.Error(t, NormalizeTokenSessionFailover(token, "default"))
		})
	}
}

func TestNormalizeTokenSessionFailoverSetsPrimaryGroup(t *testing.T) {
	token := &model.Token{
		Group:                       "auto",
		CrossGroupRetry:             true,
		SessionGroupFailoverEnabled: true,
		SessionFailoverGroups:       `["default","vip"]`,
		SessionFailoverThreshold:    2,
	}

	require.NoError(t, NormalizeTokenSessionFailover(token, "default"))
	require.Equal(t, "default", token.Group)
	require.False(t, token.CrossGroupRetry)
	require.JSONEq(t, `["default","vip"]`, token.SessionFailoverGroups)
}

func TestApiKeyGroupFailoverRedisKeyIsTokenScoped(t *testing.T) {
	require.Equal(t, "new-api:api_key_group_failover:v1:123", apiKeyGroupFailoverRedisKey(123))
}

func TestResetApiKeyGroupFailoverStateNoopsWithoutRedis(t *testing.T) {
	require.NoError(t, ResetApiKeyGroupFailoverState(123))
}

func TestSameFailoverGroups(t *testing.T) {
	require.True(t, sameFailoverGroups([]string{"default", "vip"}, []string{"default", "vip"}))
	require.False(t, sameFailoverGroups([]string{"default", "vip"}, []string{"default", "backup"}))
	require.False(t, sameFailoverGroups([]string{"default"}, []string{"default", "vip"}))
}

func TestGetApiKeyGroupFailoverRuntimeDefaultsToP0WithoutRedis(t *testing.T) {
	token := &model.Token{
		Id:                          123,
		SessionGroupFailoverEnabled: true,
		SessionFailoverGroups:       `["default","vip"]`,
		SessionFailoverThreshold:    2,
	}

	runtime := GetApiKeyGroupFailoverRuntime(token)

	require.NotNil(t, runtime)
	require.Equal(t, 0, runtime.CurrentLevel)
	require.Equal(t, "default", runtime.SelectedGroup)
	require.Equal(t, 0, runtime.FailureCount)
	require.Equal(t, 2, runtime.Threshold)
	require.Equal(t, "api_key", runtime.Scope)
}

func TestNextSessionGroupFailoverState(t *testing.T) {
	info := SessionGroupFailoverContext{
		Groups:       []string{"default", "vip", "svip"},
		CurrentLevel: 0,
		Threshold:    2,
	}
	state := SessionGroupFailoverState{
		LevelIndex:   0,
		FailureCount: 1,
		Groups:       info.Groups,
	}

	next, switched := nextSessionGroupFailoverState(state, info, false)
	require.True(t, switched)
	require.Equal(t, 1, next.LevelIndex)
	require.Equal(t, 0, next.FailureCount)

	info.CurrentLevel = 1
	state = SessionGroupFailoverState{
		LevelIndex:   1,
		FailureCount: 1,
		Groups:       info.Groups,
	}
	next, switched = nextSessionGroupFailoverState(state, info, true)
	require.False(t, switched)
	require.Equal(t, 1, next.LevelIndex)
	require.Equal(t, 0, next.FailureCount)

	info.CurrentLevel = 0
	state = SessionGroupFailoverState{
		LevelIndex:   2,
		FailureCount: 1,
		Groups:       info.Groups,
	}
	next, switched = nextSessionGroupFailoverState(state, info, true)
	require.False(t, switched)
	require.Equal(t, 2, next.LevelIndex)
	require.Equal(t, 1, next.FailureCount)
}

func TestSelectAvailableSessionFailoverLevelSkipsClosedCurrentGroup(t *testing.T) {
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","backup":"Backup"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"backup":1}`))

	level, ok := selectAvailableSessionFailoverLevel([]string{"default", "vip", "backup"}, 1, "", "")

	require.True(t, ok)
	require.Equal(t, 2, level)
}

func TestSelectAvailableSessionFailoverLevelSkipsGroupWithoutModelChannel(t *testing.T) {
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
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
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, db.Create(&model.Ability{
		Group:     "vip",
		Model:     "gpt-test",
		ChannelId: 2,
		Enabled:   false,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "backup",
		Model:     "gpt-test",
		ChannelId: 1,
		Enabled:   true,
	}).Error)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP","backup":"Backup"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"backup":1}`))

	level, ok := selectAvailableSessionFailoverLevel([]string{"default", "vip", "backup"}, 1, "", "gpt-test")

	require.True(t, ok)
	require.Equal(t, 2, level)
}
