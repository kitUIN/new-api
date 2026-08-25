package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ResolveGroupCombinationChannel resolves a virtual billing group to its exact
// channel for the requested model. The caller keeps the virtual group in context.
func ResolveGroupCombinationChannel(group, modelName string) (*model.Channel, bool, error) {
	channelID, configured, routed := ratio_setting.GetGroupCombinationChannelID(group, modelName)
	if !configured {
		return nil, false, nil
	}
	if !routed {
		return nil, true, fmt.Errorf("组合分组 %s 未配置模型 %s 的渠道", group, modelName)
	}
	channel, err := model.CacheGetChannel(channelID)
	if err != nil || channel == nil {
		return nil, true, fmt.Errorf("组合分组 %s 配置的渠道 #%d 不存在", group, channelID)
	}
	if channel.Status != common.ChannelStatusEnabled {
		return nil, true, fmt.Errorf("组合分组 %s 配置的渠道 #%d 已停用", group, channelID)
	}
	if !model.ChannelExposesRequestedModel(channel, modelName) {
		return nil, true, fmt.Errorf("组合分组 %s 配置的渠道 #%d 不支持模型 %s", group, channelID, modelName)
	}
	return channel, true, nil
}
