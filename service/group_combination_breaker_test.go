package service

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func setupGroupCombinationBreakerTest(t *testing.T) *time.Time {
	t.Helper()
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	originalNow := groupCombinationBreakerNow
	now := time.Unix(1_800_000_000, 0)

	common.RedisEnabled = false
	common.RDB = nil
	groupCombinationBreakerNow = func() time.Time { return now }
	groupCombinationBreakerMemoryMu.Lock()
	groupCombinationBreakerMemory = make(map[string]GroupCombinationBreakerState)
	groupCombinationBreakerMemoryMu.Unlock()
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"combo-a":[{"group":"group-a","models":["model-a"]},{"group":"group-b","models":["model-a"]}],"combo-b":[{"group":"group-a","models":["model-b"]},{"group":"group-c","models":["model-b"]}]}`,
	))

	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		groupCombinationBreakerNow = originalNow
		groupCombinationBreakerMemoryMu.Lock()
		groupCombinationBreakerMemory = make(map[string]GroupCombinationBreakerState)
		groupCombinationBreakerMemoryMu.Unlock()
	})
	return &now
}

func TestGroupCombinationBreakerStateMachine(t *testing.T) {
	now := setupGroupCombinationBreakerTest(t)

	for failure := 1; failure < GroupCombinationBreakerFailureThreshold; failure++ {
		state, opened, err := updateGroupCombinationBreakerState("group-a", false)
		require.NoError(t, err)
		require.False(t, opened)
		require.Equal(t, failure, state.ConsecutiveFailures)
		require.Zero(t, state.SkippedUntil)
	}

	summary, err := GetGroupCombinationCircuitBreakerSummary()
	require.NoError(t, err)
	status := requireGroupCombinationBreakerStatus(t, summary, "group-a")
	require.Equal(t, GroupCombinationBreakerStatusWarning, status.Status)
	require.Equal(t, 4, status.ConsecutiveFailures)

	_, _, err = updateGroupCombinationBreakerState("group-a", true)
	require.NoError(t, err)
	status = requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusHealthy, status.Status)
	require.Zero(t, status.ConsecutiveFailures)

	for failure := 1; failure <= GroupCombinationBreakerFailureThreshold; failure++ {
		state, opened, err := updateGroupCombinationBreakerState("group-a", false)
		require.NoError(t, err)
		require.Equal(t, failure == GroupCombinationBreakerFailureThreshold, opened)
		if opened {
			require.Equal(t, now.Unix()+GroupCombinationBreakerCooldownSeconds, state.SkippedUntil)
		}
	}
	openedUntil := now.Unix() + GroupCombinationBreakerCooldownSeconds

	*now = now.Add(time.Minute)
	state, opened, err := updateGroupCombinationBreakerState("group-a", false)
	require.NoError(t, err)
	require.False(t, opened)
	require.Equal(t, openedUntil, state.SkippedUntil)
	state, opened, err = updateGroupCombinationBreakerState("group-a", true)
	require.NoError(t, err)
	require.False(t, opened)
	require.Equal(t, openedUntil, state.SkippedUntil)

	*now = time.Unix(openedUntil+1, 0)
	status = requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusHealthy, status.Status)
	require.Zero(t, status.ConsecutiveFailures)

	state, opened, err = updateGroupCombinationBreakerState("group-a", false)
	require.NoError(t, err)
	require.False(t, opened)
	require.Equal(t, 1, state.ConsecutiveFailures)
	resetStatus, err := ResetGroupCombinationCircuitBreaker("group-a")
	require.NoError(t, err)
	require.Equal(t, GroupCombinationBreakerStatusHealthy, resetStatus.Status)
	require.Zero(t, resetStatus.ConsecutiveFailures)
}

func TestGroupCombinationBreakerUsesMemberScopeAndInvalidatesChangedConfig(t *testing.T) {
	setupGroupCombinationBreakerTest(t)

	for failure := 0; failure < GroupCombinationBreakerFailureThreshold; failure++ {
		_, _, err := updateGroupCombinationBreakerState("group-a", false)
		require.NoError(t, err)
	}
	status := requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusSkipped, status.Status)

	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"combo-a":[{"group":"group-a","models":["model-a","model-c"]},{"group":"group-b","models":["model-a"]}]}`,
	))
	status = requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusHealthy, status.Status)
}

func TestGroupCombinationBreakerSuccessOnlyClearsSelectedMember(t *testing.T) {
	setupGroupCombinationBreakerTest(t)
	for failure := 0; failure < 2; failure++ {
		_, _, err := updateGroupCombinationBreakerState("group-a", false)
		require.NoError(t, err)
	}

	context := testGroupCombinationRuntimeContext(1)
	RecordGroupCombinationSuccess(context)
	status := requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusWarning, status.Status)
	require.Equal(t, 2, status.ConsecutiveFailures)

	context = testGroupCombinationRuntimeContext(0)
	RecordGroupCombinationSuccess(context)
	status = requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusHealthy, status.Status)
}

func testGroupCombinationRuntimeContext(selectedIndex int) *gin.Context {
	context, _ := gin.CreateTestContext(nil)
	context.Set(ginKeyGroupCombinationRuntime, &groupCombinationRuntime{
		Members: []ratio_setting.GroupCombinationMember{
			{Group: "group-a", Models: []string{"model-a"}},
			{Group: "group-b", Models: []string{"model-a"}},
		},
		SelectedIndex: selectedIndex,
	})
	return context
}

func TestGroupCombinationBreakerMemoryUpdatesAreConcurrentSafe(t *testing.T) {
	setupGroupCombinationBreakerTest(t)

	var waitGroup sync.WaitGroup
	errors := make(chan error, 32)
	for failure := 0; failure < 32; failure++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, _, err := updateGroupCombinationBreakerState("group-a", false)
			errors <- err
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}

	status := requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusSkipped, status.Status)
	require.Equal(t, GroupCombinationBreakerFailureThreshold, status.ConsecutiveFailures)
}

func TestGroupCombinationBreakerRedisWatchPreservesConcurrentFailures(t *testing.T) {
	setupGroupCombinationBreakerTest(t)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	common.RDB = redisClient
	common.RedisEnabled = true

	var waitGroup sync.WaitGroup
	errors := make(chan error, GroupCombinationBreakerFailureThreshold-1)
	for failure := 0; failure < GroupCombinationBreakerFailureThreshold-1; failure++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, _, err := updateGroupCombinationBreakerState("group-a", false)
			errors <- err
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}

	status := requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusWarning, status.Status)
	require.Equal(t, GroupCombinationBreakerFailureThreshold-1, status.ConsecutiveFailures)

	state, opened, err := updateGroupCombinationBreakerState("group-a", false)
	require.NoError(t, err)
	require.True(t, opened)
	require.Equal(t, GroupCombinationBreakerFailureThreshold, state.ConsecutiveFailures)
	require.Equal(t, time.Duration(GroupCombinationBreakerCooldownSeconds)*time.Second, redisServer.TTL(groupCombinationBreakerRedisKey("group-a")))

	redisServer.FastForward(time.Minute)
	_, opened, err = updateGroupCombinationBreakerState("group-a", false)
	require.NoError(t, err)
	require.False(t, opened)
	require.Equal(t, time.Duration(GroupCombinationBreakerCooldownSeconds)*time.Second-time.Minute, redisServer.TTL(groupCombinationBreakerRedisKey("group-a")))

	originalConfiguration := ratio_setting.GroupCombinations2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"combo-a":[{"group":"group-a","models":["model-a","model-c"]},{"group":"group-b","models":["model-a"]}]}`,
	))
	status = requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusHealthy, status.Status)
	require.False(t, redisServer.Exists(groupCombinationBreakerRedisKey("group-a")))
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalConfiguration))
	status = requireGroupCombinationBreakerStatus(t, mustGroupCombinationBreakerSummary(t), "group-a")
	require.Equal(t, GroupCombinationBreakerStatusHealthy, status.Status)
}

func TestGroupCombinationBreakerCacheFailureIsFailOpen(t *testing.T) {
	setupGroupCombinationBreakerTest(t)
	redisClient := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  20 * time.Millisecond,
		ReadTimeout:  20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = redisClient.Close() })
	common.RDB = redisClient
	common.RedisEnabled = true

	require.False(t, isGroupCombinationMemberSkipped("group-a"))
	_, _, err := updateGroupCombinationBreakerState("group-a", false)
	require.Error(t, err)
}

func mustGroupCombinationBreakerSummary(t *testing.T) GroupCombinationBreakerSummary {
	t.Helper()
	summary, err := GetGroupCombinationCircuitBreakerSummary()
	require.NoError(t, err)
	return summary
}

func requireGroupCombinationBreakerStatus(t *testing.T, summary GroupCombinationBreakerSummary, group string) GroupCombinationBreakerStatus {
	t.Helper()
	for _, status := range summary.Groups {
		if status.Group == group {
			return status
		}
	}
	t.Fatalf("group %s not found in breaker summary: %+v", group, summary.Groups)
	return GroupCombinationBreakerStatus{}
}
