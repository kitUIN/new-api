package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBillingAuditExclusionsPersistPerMonth(t *testing.T) {
	setupBillingCostTest(t)
	require.NoError(t, DB.AutoMigrate(&BillingAuditPreference{}))
	groups, err := GetBillingAuditExcludedGroups("2026-09")
	require.NoError(t, err)
	require.Equal(t, []string{}, groups)
	require.NoError(t, SaveBillingAuditExcludedGroups("2026-09", []string{"internal", ""}, 1))
	require.NoError(t, SaveBillingAuditExcludedGroups("2026-10", []string{"test"}, 2))
	groups, err = GetBillingAuditExcludedGroups("2026-09")
	require.NoError(t, err)
	require.Equal(t, []string{"internal", ""}, groups)
	// Updating and clearing a month must not change another month's selection.
	require.NoError(t, SaveBillingAuditExcludedGroups("2026-09", []string{"other"}, 2))
	groups, err = GetBillingAuditExcludedGroups("2026-09")
	require.NoError(t, err)
	require.Equal(t, []string{"other"}, groups)
	require.NoError(t, SaveBillingAuditExcludedGroups("2026-09", []string{}, 1))
	groups, err = GetBillingAuditExcludedGroups("2026-09")
	require.NoError(t, err)
	require.Equal(t, []string{}, groups)
	groups, err = GetBillingAuditExcludedGroups("2026-10")
	require.NoError(t, err)
	require.Equal(t, []string{"test"}, groups)
	var count int64
	require.NoError(t, DB.Model(&BillingAuditPreference{}).Count(&count).Error)
	require.Equal(t, int64(2), count)
}
