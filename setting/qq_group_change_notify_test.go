package setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSendQQGroupChangeNotificationDefaultsToGroup(t *testing.T) {
	originalSetting := qqGroupChangeNotifySetting
	originalAdminFunc := sendQQAdminMessageFunc
	originalGroupFunc := sendQQGroupMessageFunc
	t.Cleanup(func() {
		qqGroupChangeNotifySetting = originalSetting
		sendQQAdminMessageFunc = originalAdminFunc
		sendQQGroupMessageFunc = originalGroupFunc
	})

	var adminCalls int
	var groupCalls int
	sendQQAdminMessageFunc = func(string, string) {
		adminCalls++
	}
	sendQQGroupMessageFunc = func(string, string) {
		groupCalls++
	}
	qqGroupChangeNotifySetting = QQGroupChangeNotifySetting{
		GroupRatioChangeEnabled:      true,
		GroupRatioChangeTarget:       QQGroupChangeNotifyTargetGroup,
		BindingChangeEnabled:         true,
		BindingChangeTarget:          QQGroupChangeNotifyTargetGroup,
		UserUsableGroupChangeEnabled: true,
		UserUsableGroupChangeTarget:  QQGroupChangeNotifyTargetGroup,
	}

	SendQQGroupChangeNotification(QQGroupChangeNotifyEventGroupRatio, "message", "test notify")
	SendQQGroupChangeNotification(QQGroupChangeNotifyEventBinding, "message", "test notify")
	SendQQGroupChangeNotification(QQGroupChangeNotifyEventUserUsableGroup, "message", "test notify")

	require.Equal(t, 0, adminCalls)
	require.Equal(t, 3, groupCalls)
}

func TestSendQQGroupChangeNotificationSkipsDisabledEvent(t *testing.T) {
	originalSetting := qqGroupChangeNotifySetting
	originalAdminFunc := sendQQAdminMessageFunc
	originalGroupFunc := sendQQGroupMessageFunc
	t.Cleanup(func() {
		qqGroupChangeNotifySetting = originalSetting
		sendQQAdminMessageFunc = originalAdminFunc
		sendQQGroupMessageFunc = originalGroupFunc
	})

	var calls int
	sendQQAdminMessageFunc = func(string, string) {
		calls++
	}
	sendQQGroupMessageFunc = func(string, string) {
		calls++
	}
	qqGroupChangeNotifySetting = QQGroupChangeNotifySetting{
		GroupRatioChangeEnabled: false,
		GroupRatioChangeTarget:  QQGroupChangeNotifyTargetBoth,
	}

	SendQQGroupChangeNotification(QQGroupChangeNotifyEventGroupRatio, "message", "test notify")

	require.Equal(t, 0, calls)
}

func TestSendQQGroupChangeNotificationTargets(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		wantAdmin int
		wantGroup int
	}{
		{name: "admin", target: QQGroupChangeNotifyTargetAdmin, wantAdmin: 1},
		{name: "group", target: QQGroupChangeNotifyTargetGroup, wantGroup: 1},
		{name: "both", target: QQGroupChangeNotifyTargetBoth, wantAdmin: 1, wantGroup: 1},
		{name: "empty target defaults to group", target: "", wantGroup: 1},
		{name: "invalid target defaults to group", target: "invalid", wantGroup: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSetting := qqGroupChangeNotifySetting
			originalAdminFunc := sendQQAdminMessageFunc
			originalGroupFunc := sendQQGroupMessageFunc
			t.Cleanup(func() {
				qqGroupChangeNotifySetting = originalSetting
				sendQQAdminMessageFunc = originalAdminFunc
				sendQQGroupMessageFunc = originalGroupFunc
			})

			var adminCalls int
			var groupCalls int
			sendQQAdminMessageFunc = func(string, string) {
				adminCalls++
			}
			sendQQGroupMessageFunc = func(string, string) {
				groupCalls++
			}
			qqGroupChangeNotifySetting = QQGroupChangeNotifySetting{
				BindingChangeEnabled: true,
				BindingChangeTarget:  tt.target,
			}

			SendQQGroupChangeNotification(QQGroupChangeNotifyEventBinding, "message", "test notify")

			require.Equal(t, tt.wantAdmin, adminCalls)
			require.Equal(t, tt.wantGroup, groupCalls)
		})
	}
}
