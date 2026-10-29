package service

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/types"
)

func SetUpstreamUsageExhaustedMessage(group string, apiErr *types.NewAPIError) {
	if apiErr == nil || apiErr.UpstreamStatusCode != http.StatusServiceUnavailable {
		return
	}
	if !strings.Contains(strings.ToLower(apiErr.Error()+" "+apiErr.ResponseBody), "no available accounts") {
		return
	}
	if group == "" {
		return
	}
	providers, err := ListUpstreamUsage()
	if err != nil {
		return
	}
	accounts := []UpstreamUsageAccountView{}
	for _, provider := range providers {
		for _, account := range provider.Accounts {
			if slices.Contains(account.Groups, group) {
				accounts = append(accounts, account)
			}
		}
	}
	reset, exhausted := exhaustedUpstreamUsageReset(accounts, time.Now())
	if !exhausted {
		return
	}
	resetText := "暂不可用，请稍后重试"
	if !reset.IsZero() {
		resetText = reset.In(time.FixedZone("UTC+8", 8*60*60)).Format("2006-01-02 15:04:05") + "（UTC+8）"
	}
	apiErr.ResponseMessage = fmt.Sprintf("限额已用光，下次重置时间：%s", resetText)
}

// A group is exhausted only when every bound account is blocked by a usage
// window. Each account must wait for all exhausted windows to reset; the group
// can recover as soon as the first account becomes available.
func exhaustedUpstreamUsageReset(accounts []UpstreamUsageAccountView, now time.Time) (time.Time, bool) {
	if len(accounts) == 0 {
		return time.Time{}, false
	}
	var nextReset time.Time
	unknownReset := false
	for _, account := range accounts {
		if !account.Available || account.Usage == nil || !validUpstreamUsageWindow(account.Usage.SevenDay) {
			return time.Time{}, false
		}
		var accountReset time.Time
		blocked := false
		for _, window := range []*UpstreamUsageWindow{account.Usage.FiveHour, account.Usage.SevenDay} {
			if window == nil {
				continue
			}
			if !validUpstreamUsageWindow(window) {
				return time.Time{}, false
			}
			if *window.Utilization < 100 {
				continue
			}
			reset, err := time.Parse(time.RFC3339, window.ResetsAt)
			if err != nil && window.RemainingSeconds > 0 && account.FetchedAt > 0 {
				reset = time.Unix(account.FetchedAt+window.RemainingSeconds, 0)
			}
			// A snapshot from before a reset no longer proves exhaustion.
			if !reset.IsZero() && !reset.After(now) {
				continue
			}
			blocked = true
			if reset.IsZero() {
				unknownReset = true
			} else if reset.After(accountReset) {
				accountReset = reset
			}
		}
		if !blocked {
			return time.Time{}, false
		}
		if nextReset.IsZero() || accountReset.Before(nextReset) {
			nextReset = accountReset
		}
	}
	if unknownReset {
		return time.Time{}, true
	}
	return nextReset, true
}

func validUpstreamUsageWindow(window *UpstreamUsageWindow) bool {
	return window != nil && window.Utilization != nil && !math.IsNaN(*window.Utilization) && *window.Utilization >= 0 && *window.Utilization <= 100
}
