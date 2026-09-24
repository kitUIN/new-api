package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupOpeningHoursBlocksRetryAndContextRoutes(t *testing.T) {
	config := ratio_setting.GetGroupRatioSetting().GroupOpeningHours
	original, err := common.Marshal(config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, common.Unmarshal(original, config)) })
	now := time.Now().In(time.FixedZone("UTC+8", 8*3600))
	value, err := common.Marshal(map[string][]ratio_setting.GroupOpeningInterval{
		"closed": {{Start: now.Add(time.Hour).Format("15:04"), End: now.Add(2 * time.Hour).Format("15:04")}},
	})
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(value, config))
	for _, key := range []constant.ContextKey{constant.ContextKeyTokenGroup, constant.ContextKeyUsingGroup, constant.ContextKeyAutoGroup, constant.ContextKeyGroupCombination} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		common.SetContextKey(c, key, "closed")
		var closed *ratio_setting.GroupNotOpenError
		require.ErrorAs(t, CheckRequestGroupOpeningHours(c), &closed)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	for retry := 0; retry < 3; retry++ {
		channel, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: c, TokenGroup: "closed", ModelName: "test", Retry: &retry})
		require.Nil(t, channel)
		require.Equal(t, "closed", group)
		var closed *ratio_setting.GroupNotOpenError
		require.ErrorAs(t, err, &closed)
	}
}
