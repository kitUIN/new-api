package service

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupApiKeyFailoverRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	originalRDB, originalEnabled := common.RDB, common.RedisEnabled
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() {
		_ = client.Close()
		common.RDB, common.RedisEnabled = originalRDB, originalEnabled
	})
	return server
}

func failoverTestToken(id int) *model.Token {
	return &model.Token{
		Id:                          id,
		SessionGroupFailoverEnabled: true,
		SessionFailoverGroups:       `["default","vip"]`,
		SessionFailoverThreshold:    2,
	}
}

func failoverTestContext(t *testing.T, token *model.Token) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	state, found := readSessionGroupFailoverState(apiKeyGroupFailoverRedisKey(token.Id))
	require.True(t, found)
	common.SetContextKey(c, constant.ContextKeySessionGroupFailover, &SessionGroupFailoverContext{
		Enabled:      true,
		Groups:       state.Groups,
		CurrentLevel: state.LevelIndex,
		FailureCount: state.FailureCount,
		Threshold:    token.SessionFailoverThreshold,
		RedisKey:     apiKeyGroupFailoverRedisKey(token.Id),
		Generation:   state.Generation,
	})
	return c
}

func promoteFailoverTestToken(t *testing.T, token *model.Token) {
	t.Helper()
	require.NoError(t, TouchApiKeyGroupFailoverState(token))
	for i := 0; i < token.SessionFailoverThreshold; i++ {
		RecordSessionGroupFailoverResult(failoverTestContext(t, token), false)
	}
	require.Equal(t, 1, GetApiKeyGroupFailoverRuntime(token).CurrentLevel)
}

func TestApiKeyFailoverResetsAfterTwoIdleHours(t *testing.T) {
	server := setupApiKeyFailoverRedis(t)
	token := failoverTestToken(123)
	promoteFailoverTestToken(t, token)
	key := apiKeyGroupFailoverRedisKey(token.Id)
	ttl := server.TTL(key)
	require.InDelta(t, (2 * time.Hour).Milliseconds(), ttl.Milliseconds(), 1000)

	server.FastForward(ttl - time.Millisecond)
	require.Equal(t, 1, GetApiKeyGroupFailoverRuntime(token).CurrentLevel)
	// Looking at the API key page must not extend the idle timeout.
	require.Equal(t, time.Millisecond, server.TTL(key))
	server.FastForward(time.Millisecond)
	runtime := GetApiKeyGroupFailoverRuntime(token)
	require.Equal(t, 0, runtime.CurrentLevel)
	require.Zero(t, runtime.FailureCount)
	require.Equal(t, "default", runtime.SelectedGroup)
	require.NoError(t, TouchApiKeyGroupFailoverState(token))
	require.Equal(t, 0, GetApiKeyGroupFailoverRuntime(token).CurrentLevel)
}

func TestApiKeyFailoverRequestsExtendOnlyTheirOwnIdleTimeout(t *testing.T) {
	server := setupApiKeyFailoverRedis(t)
	token, otherToken := failoverTestToken(123), failoverTestToken(456)
	promoteFailoverTestToken(t, token)
	promoteFailoverTestToken(t, otherToken)
	server.FastForward(90 * time.Minute)
	require.NoError(t, TouchApiKeyGroupFailoverState(token))
	require.Equal(t, 1, GetApiKeyGroupFailoverRuntime(token).CurrentLevel)
	server.FastForward(31 * time.Minute)
	require.Equal(t, 1, GetApiKeyGroupFailoverRuntime(token).CurrentLevel)
	require.Equal(t, 0, GetApiKeyGroupFailoverRuntime(otherToken).CurrentLevel)
	server.FastForward(90 * time.Minute)
	require.Equal(t, 0, GetApiKeyGroupFailoverRuntime(token).CurrentLevel)
}

func TestApplySessionGroupFailoverRestartsAtP0AfterIdle(t *testing.T) {
	server := setupApiKeyFailoverRedis(t)
	originalGroups := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	token := failoverTestToken(123)
	promoteFailoverTestToken(t, token)
	apply := func() *gin.Context {
		require.NoError(t, TouchApiKeyGroupFailoverState(token))
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		common.SetContextKey(c, constant.ContextKeyTokenId, token.Id)
		common.SetContextKey(c, constant.ContextKeyTokenSessionGroupFailoverEnabled, true)
		common.SetContextKey(c, constant.ContextKeyTokenSessionFailoverGroups, token.SessionFailoverGroups)
		common.SetContextKey(c, constant.ContextKeyTokenSessionFailoverThreshold, token.SessionFailoverThreshold)
		ApplySessionGroupFailover(c, "")
		return c
	}
	require.Equal(t, "vip", common.GetContextKeyString(apply(), constant.ContextKeyUsingGroup))
	server.FastForward(2 * time.Hour)
	c := apply()
	require.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	RecordSessionGroupFailoverResult(c, false)
	require.Equal(t, 1, GetApiKeyGroupFailoverRuntime(token).FailureCount)
}

func TestApiKeyFailoverResultsDoNotExtendIdleTimeout(t *testing.T) {
	server := setupApiKeyFailoverRedis(t)
	token := failoverTestToken(123)
	promoteFailoverTestToken(t, token)
	key := apiKeyGroupFailoverRedisKey(token.Id)
	state, found := readSessionGroupFailoverState(key)
	require.True(t, found)
	state.LastRequestAt = common.GetTimestamp() - 3600
	state.FailureCount = 1
	data, err := common.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, common.RedisSet(key, string(data), time.Hour))

	RecordSessionGroupFailoverResult(failoverTestContext(t, token), true)
	runtime := GetApiKeyGroupFailoverRuntime(token)
	require.Equal(t, 1, runtime.CurrentLevel)
	require.Zero(t, runtime.FailureCount)
	require.InDelta(t, time.Hour.Milliseconds(), server.TTL(key).Milliseconds(), 1000)
}

func TestApiKeyFailoverIgnoresResultsFromBeforeReset(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprintf("manual=%t", manual), func(t *testing.T) {
			server := setupApiKeyFailoverRedis(t)
			token := failoverTestToken(123)
			promoteFailoverTestToken(t, token)
			oldRequest := failoverTestContext(t, token)
			if manual {
				require.NoError(t, ResetApiKeyGroupFailoverState(token.Id))
			} else {
				server.FastForward(2 * time.Hour)
			}
			RecordSessionGroupFailoverResult(oldRequest, false)
			require.False(t, server.Exists(apiKeyGroupFailoverRedisKey(token.Id)))
			require.NoError(t, TouchApiKeyGroupFailoverState(token))
			RecordSessionGroupFailoverResult(oldRequest, true)
			runtime := GetApiKeyGroupFailoverRuntime(token)
			require.Equal(t, 0, runtime.CurrentLevel)
			require.Zero(t, runtime.FailureCount)
		})
	}
}

func TestApiKeyFailoverMigratesPermanentStates(t *testing.T) {
	for _, idle := range []time.Duration{time.Hour, 2 * time.Hour} {
		t.Run(idle.String(), func(t *testing.T) {
			server := setupApiKeyFailoverRedis(t)
			token := failoverTestToken(123)
			state := SessionGroupFailoverState{
				LevelIndex: 1, FailureCount: 1, Groups: []string{"default", "vip"},
				UpdatedAt: common.GetTimestamp() - int64(idle/time.Second),
			}
			data, err := common.Marshal(state)
			require.NoError(t, err)
			key := apiKeyGroupFailoverRedisKey(token.Id)
			require.NoError(t, common.RedisSet(key, string(data), 0))
			expectedLevel := 1
			if idle >= 2*time.Hour {
				expectedLevel = 0
			}
			require.Equal(t, expectedLevel, GetApiKeyGroupFailoverRuntime(token).CurrentLevel)
			require.NoError(t, TouchApiKeyGroupFailoverState(token))
			require.Equal(t, expectedLevel, GetApiKeyGroupFailoverRuntime(token).CurrentLevel)
			require.Positive(t, server.TTL(key))
		})
	}
}

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
