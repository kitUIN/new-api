package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
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
