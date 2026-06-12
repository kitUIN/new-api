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
