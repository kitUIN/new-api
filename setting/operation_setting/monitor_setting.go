package operation_setting

import (
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled    bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes    float64 `json:"auto_test_channel_minutes"`
	AutoTestChannelSkipGroups string  `json:"auto_test_channel_skip_groups"`
}

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:    false,
	AutoTestChannelMinutes:    10,
	AutoTestChannelSkipGroups: "",
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
		}
	}
	return &monitorSetting
}

func ParseAutoTestChannelSkipGroups(raw string) map[string]struct{} {
	groups := make(map[string]struct{})
	for _, group := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\r' || r == '\t'
	}) {
		group = strings.TrimSpace(group)
		if group != "" {
			groups[group] = struct{}{}
		}
	}
	return groups
}

func ShouldSkipAutoTestChannelGroup(group string) bool {
	group = strings.TrimSpace(group)
	if group == "" {
		return false
	}
	_, ok := ParseAutoTestChannelSkipGroups(GetMonitorSetting().AutoTestChannelSkipGroups)[group]
	return ok
}
