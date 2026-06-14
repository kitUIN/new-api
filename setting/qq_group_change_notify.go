package setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const (
	QQGroupChangeNotifyTargetAdmin = "admin"
	QQGroupChangeNotifyTargetGroup = "group"
	QQGroupChangeNotifyTargetBoth  = "both"
)

type QQGroupChangeNotifyEvent string

const (
	QQGroupChangeNotifyEventGroupRatio      QQGroupChangeNotifyEvent = "group_ratio"
	QQGroupChangeNotifyEventBinding         QQGroupChangeNotifyEvent = "binding"
	QQGroupChangeNotifyEventUserUsableGroup QQGroupChangeNotifyEvent = "user_usable_group"
)

type QQGroupChangeNotifySetting struct {
	GroupRatioChangeEnabled      bool   `json:"group_ratio_change_enabled"`
	GroupRatioChangeTarget       string `json:"group_ratio_change_target"`
	BindingChangeEnabled         bool   `json:"binding_change_enabled"`
	BindingChangeTarget          string `json:"binding_change_target"`
	UserUsableGroupChangeEnabled bool   `json:"user_usable_group_change_enabled"`
	UserUsableGroupChangeTarget  string `json:"user_usable_group_change_target"`
}

var qqGroupChangeNotifySetting = QQGroupChangeNotifySetting{
	GroupRatioChangeEnabled:      true,
	GroupRatioChangeTarget:       QQGroupChangeNotifyTargetGroup,
	BindingChangeEnabled:         true,
	BindingChangeTarget:          QQGroupChangeNotifyTargetGroup,
	UserUsableGroupChangeEnabled: true,
	UserUsableGroupChangeTarget:  QQGroupChangeNotifyTargetGroup,
}

var (
	sendQQAdminMessageFunc = common.SendQQAdminMessage
	sendQQGroupMessageFunc = common.SendQQNotificationGroupMessage
)

func init() {
	config.GlobalConfig.Register("qq_group_change_notify_setting", &qqGroupChangeNotifySetting)
}

func GetQQGroupChangeNotifySetting() QQGroupChangeNotifySetting {
	return qqGroupChangeNotifySetting
}

func normalizeQQGroupChangeNotifyTarget(target string) string {
	switch strings.TrimSpace(target) {
	case QQGroupChangeNotifyTargetAdmin:
		return QQGroupChangeNotifyTargetAdmin
	case QQGroupChangeNotifyTargetBoth:
		return QQGroupChangeNotifyTargetBoth
	default:
		return QQGroupChangeNotifyTargetGroup
	}
}

func getQQGroupChangeNotifyEventConfig(event QQGroupChangeNotifyEvent) (bool, string) {
	switch event {
	case QQGroupChangeNotifyEventGroupRatio:
		return qqGroupChangeNotifySetting.GroupRatioChangeEnabled, qqGroupChangeNotifySetting.GroupRatioChangeTarget
	case QQGroupChangeNotifyEventBinding:
		return qqGroupChangeNotifySetting.BindingChangeEnabled, qqGroupChangeNotifySetting.BindingChangeTarget
	case QQGroupChangeNotifyEventUserUsableGroup:
		return qqGroupChangeNotifySetting.UserUsableGroupChangeEnabled, qqGroupChangeNotifySetting.UserUsableGroupChangeTarget
	default:
		return false, QQGroupChangeNotifyTargetGroup
	}
}

func SendQQGroupChangeNotification(event QQGroupChangeNotifyEvent, message string, logPrefix string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	enabled, rawTarget := getQQGroupChangeNotifyEventConfig(event)
	if !enabled {
		return
	}

	target := normalizeQQGroupChangeNotifyTarget(rawTarget)
	if target == QQGroupChangeNotifyTargetAdmin || target == QQGroupChangeNotifyTargetBoth {
		sendQQAdminMessageFunc(message, logPrefix)
	}
	if target == QQGroupChangeNotifyTargetGroup || target == QQGroupChangeNotifyTargetBoth {
		sendQQGroupMessageFunc(message, logPrefix)
	}
}
