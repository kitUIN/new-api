package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDrawingResolutionCombinationRouting(t *testing.T) {
	setupModelGroupCombinationDatabase(t)
	original := ratio_setting.GetGroupRatioSetting().GroupDrawingResolutions
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		ratio_setting.GetGroupRatioSetting().GroupDrawingResolutions = original
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})
	config := &ratio_setting.GroupDrawingResolutionsConfig{}
	ratio_setting.GetGroupRatioSetting().GroupDrawingResolutions = config
	require.NoError(t, common.Unmarshal([]byte(`{"drawing-a":["1K"],"drawing-b":["2K"],"drawing-c":["2K"]}`), config))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-image":1,"drawing-a":1,"drawing-b":2,"drawing-c":3}`))
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(`{"gpt-image":[{"group":"drawing-a","models":["image","only-1k"]},{"group":"drawing-b","models":["image"]},{"group":"drawing-c","models":["image"]}]}`))
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
	for i, group := range []string{"drawing-a", "drawing-b", "drawing-c"} {
		channel := model.Channel{Id: i + 10, Name: group, Key: "test", Models: "image,only-1k", Group: group, Status: common.ChannelStatusEnabled}
		require.NoError(t, model.DB.Create(&channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	require.Equal(t, []string{"image", "only-1k"}, GetDrawingModels("gpt-image", "1K"))
	require.Equal(t, []string{"image"}, GetDrawingModels("gpt-image", "2K"))
	require.Empty(t, GetDrawingModels("gpt-image", "4K"))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	channel, group, err := ResolveDrawingChannel(ctx, "gpt-image", "image", "3584x2048")
	require.NoError(t, err)
	require.Equal(t, "drawing-b", group)
	require.Equal(t, 11, channel.Id)
	info := &relaycommon.RelayInfo{TokenGroup: "gpt-image", UsingGroup: group}
	require.Equal(t, 2.0, helper.HandleGroupRatio(ctx, info).GroupRatio)
	require.Equal(t, "drawing-b", common.GetContextKeyString(ctx, constant.ContextKeyUsingGroup))
	require.Equal(t, "gpt-image", common.GetContextKeyString(ctx, constant.ContextKeyGroupCombination))

	// Excluding B on retry must go to the next 2K member, never the 1K member A.
	channel, group, err = CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: ctx, TokenGroup: "gpt-image", ModelName: "image", ExcludedChannelIDs: []int{11}})
	require.NoError(t, err)
	require.Equal(t, "drawing-c", group)
	require.Equal(t, 12, channel.Id)
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	channel, _, err = ResolveDrawingChannel(ctx, "gpt-image", "image", "4096x4096")
	require.Error(t, err)
	require.Nil(t, channel)
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	channel, group, err = ResolveDrawingChannel(ctx, "gpt-image", "image", "1024x1792")
	require.NoError(t, err)
	require.Equal(t, "drawing-a", group)
	require.Equal(t, 10, channel.Id)
	ctx, _ = gin.CreateTestContext(httptest.NewRecorder())
	_, _, err = ResolveDrawingChannel(ctx, "drawing-a", "image", "2048x2048")
	require.ErrorContains(t, err, "不支持 2K")
}
