package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRelaySessionActivityPreservesOverrideAndExpires(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&RelaySession{}))
	now := time.Now().Unix()
	session := &RelaySession{ID: t.Name(), UserID: 7, TokenID: 11, SessionKey: "conversation", LastGroup: "A", LastSeenAt: now, CreatedAt: now}
	t.Cleanup(func() { DB.Where("id = ?", session.ID).Delete(&RelaySession{}) })
	require.NoError(t, TouchRelaySession(session))
	updated, err := UpdateRelaySessionGroup(session.ID, "B")
	require.NoError(t, err)
	require.True(t, updated)

	// A request holding old activity data must never overwrite the manual setting.
	require.NoError(t, TouchRelaySession(session))
	require.NoError(t, RecordRelaySessionGroup(session.ID, "A", now))
	stored, err := GetActiveRelaySession(session.ID)
	require.NoError(t, err)
	require.Equal(t, "B", stored.OverrideGroup)
	require.Equal(t, "A", stored.LastGroup)

	require.NoError(t, DB.Model(&RelaySession{}).Where("id = ?", session.ID).Update("last_seen_at", now-RelaySessionIdleSeconds).Error)
	_, err = GetActiveRelaySession(session.ID)
	require.Error(t, err)
	updated, err = UpdateRelaySessionGroup(session.ID, "A")
	require.NoError(t, err)
	require.False(t, updated)
	require.NoError(t, TouchRelaySession(session))
	stored, err = GetActiveRelaySession(session.ID)
	require.NoError(t, err)
	require.Empty(t, stored.OverrideGroup)
}

func TestRelaySessionListScopesBeforeSearchAndPagination(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&RelaySession{}))
	now := time.Now().Unix()
	sessions := []RelaySession{
		{ID: "scope-owner", UserID: 17, SessionKey: "same", LastSeenAt: now},
		{ID: "scope-other", UserID: 18, SessionKey: "same", LastSeenAt: now},
		{ID: "scope-expired", UserID: 17, SessionKey: "same", LastSeenAt: now - RelaySessionIdleSeconds},
	}
	require.NoError(t, DB.Create(&sessions).Error)
	t.Cleanup(func() {
		DB.Where("id IN ?", []string{"scope-owner", "scope-other", "scope-expired"}).Delete(&RelaySession{})
	})
	items, total, err := GetActiveRelaySessions(17, "same", 0, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, 17, items[0].UserID)
	items, total, err = GetActiveRelaySessions(0, "same", 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 1)
}
