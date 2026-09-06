package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestGroupCombinationCircuitBreakerHandlers(t *testing.T) {
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(
		`{"combo":[{"group":"group-a","models":["model-a"]},{"group":"group-b","models":["model-a"]}]}`,
	))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
	})

	getRecorder := httptest.NewRecorder()
	getContext, _ := gin.CreateTestContext(getRecorder)
	GetGroupCombinationCircuitBreakers(getContext)
	require.Equal(t, http.StatusOK, getRecorder.Code)
	var getResponse struct {
		Success bool                                   `json:"success"`
		Data    service.GroupCombinationBreakerSummary `json:"data"`
	}
	require.NoError(t, common.Unmarshal(getRecorder.Body.Bytes(), &getResponse))
	require.True(t, getResponse.Success)
	require.Equal(t, service.GroupCombinationBreakerFailureThreshold, getResponse.Data.FailureThreshold)
	require.Equal(t, service.GroupCombinationBreakerCooldownSeconds, getResponse.Data.CooldownSeconds)
	require.Len(t, getResponse.Data.Groups, 2)

	resetRecorder := httptest.NewRecorder()
	resetContext, _ := gin.CreateTestContext(resetRecorder)
	resetContext.Request = httptest.NewRequest(http.MethodPost, "/api/option/group_combination_circuit_breakers/reset", bytes.NewBufferString(`{"group":"group-a"}`))
	ResetGroupCombinationCircuitBreaker(resetContext)
	require.Equal(t, http.StatusOK, resetRecorder.Code)
	require.Contains(t, resetRecorder.Body.String(), `"success":true`)

	repeatedResetRecorder := httptest.NewRecorder()
	repeatedResetContext, _ := gin.CreateTestContext(repeatedResetRecorder)
	repeatedResetContext.Request = httptest.NewRequest(http.MethodPost, "/api/option/group_combination_circuit_breakers/reset", bytes.NewBufferString(`{"group":"group-a"}`))
	ResetGroupCombinationCircuitBreaker(repeatedResetContext)
	require.Equal(t, http.StatusOK, repeatedResetRecorder.Code)
	require.Contains(t, repeatedResetRecorder.Body.String(), `"success":true`)

	invalidRecorder := httptest.NewRecorder()
	invalidContext, _ := gin.CreateTestContext(invalidRecorder)
	invalidContext.Request = httptest.NewRequest(http.MethodPost, "/api/option/group_combination_circuit_breakers/reset", bytes.NewBufferString(`{"group":"missing"}`))
	ResetGroupCombinationCircuitBreaker(invalidContext)
	require.Equal(t, http.StatusBadRequest, invalidRecorder.Code)

	brokenRedis := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  20 * time.Millisecond,
		ReadTimeout:  20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = brokenRedis.Close() })
	common.RDB = brokenRedis
	common.RedisEnabled = true
	cacheErrorRecorder := httptest.NewRecorder()
	cacheErrorContext, _ := gin.CreateTestContext(cacheErrorRecorder)
	GetGroupCombinationCircuitBreakers(cacheErrorContext)
	require.Equal(t, http.StatusOK, cacheErrorRecorder.Code)
	require.Contains(t, cacheErrorRecorder.Body.String(), `"success":false`)
}
