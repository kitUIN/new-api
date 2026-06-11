package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGroupUserRequestActivity(t *testing.T) {
	group := "activity-test"
	at := time.Now().Add(-time.Minute).Truncate(time.Nanosecond)

	RecordGroupUserRequest("", at)
	_, ok := GetGroupLastUserRequest("")
	require.False(t, ok)

	RecordGroupUserRequest(group, at)
	last, ok := GetGroupLastUserRequest(group)
	require.True(t, ok)
	require.Equal(t, at, last)
}
