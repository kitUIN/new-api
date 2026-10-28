package service

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const drawingResolutionContextKey = "drawing_resolution"

func drawingMemberSupportsResolution(c *gin.Context, group string) bool {
	return c == nil || c.GetString(drawingResolutionContextKey) == "" || ratio_setting.GroupSupportsDrawingResolution(group, c.GetString(drawingResolutionContextKey))
}

func GetDrawingModels(group, resolution string) []string {
	result := make([]string, 0)
	if !ratio_setting.GroupSupportsDrawingResolution(group, resolution) {
		return result
	}
	members, combination := ratio_setting.GetGroupCombinationMembers(group)
	for _, name := range model.GetGroupEnabledModels(group) {
		if !combination {
			result = append(result, name)
			continue
		}
		for _, member := range groupCombinationMembersForModel(group, name, members, true) {
			if ratio_setting.GroupSupportsDrawingResolution(member.Group, resolution) && model.HasAvailableChannelForConcreteGroupModel(member.Group, name) {
				result = append(result, name)
				break
			}
		}
	}
	sort.Strings(result)
	return result
}

// Use the normal combination runtime so retries keep the same resolution filter.
func ResolveDrawingChannel(c *gin.Context, group, modelName, size string) (*model.Channel, string, error) {
	resolution, err := ratio_setting.DrawingResolutionForSize(size)
	if err != nil {
		return nil, group, err
	}
	c.Set(drawingResolutionContextKey, resolution)
	if !ratio_setting.GroupSupportsDrawingResolution(group, resolution) {
		return nil, group, fmt.Errorf("分组 %s 不支持 %s 绘图", group, resolution)
	}
	return CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: c, TokenGroup: group, ModelName: modelName})
}

// Exposed separately for request validation without allocating a channel.
func IsDrawingModelAvailable(group, modelName, resolution string) bool {
	return common.StringsContains(GetDrawingModels(group, resolution), modelName)
}
