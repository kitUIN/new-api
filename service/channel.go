package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

var autoDisableGroupCloseMutex sync.Mutex

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
		closeUnavailableGroupsAfterChannelDisabled(channelError.ChannelId, channelError.ChannelName, reason)
	}
}

func closeUnavailableGroupsAfterChannelDisabled(channelId int, channelName string, reason string) {
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to check disabled channel groups: channel_id=%d, error=%v", channelId, err))
		return
	}
	if channel.Status == common.ChannelStatusEnabled {
		return
	}

	autoDisableGroupCloseMutex.Lock()
	defer autoDisableGroupCloseMutex.Unlock()

	for _, group := range channel.GetGroups() {
		hasEnabledChannel, err := model.HasEnabledChannelInGroup(group)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to check enabled channels for group: group=%s, channel_id=%d, error=%v", group, channelId, err))
			continue
		}
		if hasEnabledChannel {
			continue
		}

		nextUserUsableGroups, changed, err := setting.BuildDisabledUserUsableGroupJSONString(group)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to build disabled user usable group option: group=%s, channel_id=%d, error=%v", group, channelId, err))
			continue
		}
		if !changed {
			continue
		}

		if err := model.UpdateOption("UserUsableGroups", nextUserUsableGroups); err != nil {
			common.SysLog(fmt.Sprintf("failed to close user usable group after channel disabled: group=%s, channel_id=%d, error=%v", group, channelId, err))
			continue
		}
		common.SysLog(fmt.Sprintf("分组「%s」已自动关闭：通道「%s」（#%d）自动禁用后分组下无可用渠道，原因：%s", group, channelName, channelId, reason))
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
