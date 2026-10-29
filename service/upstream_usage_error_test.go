package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestExhaustedUpstreamUsageReset(t *testing.T) {
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	window := func(used float64, hours int) *UpstreamUsageWindow {
		return &UpstreamUsageWindow{Utilization: &used, ResetsAt: now.Add(time.Duration(hours) * time.Hour).Format(time.RFC3339)}
	}
	account := func(week, short *UpstreamUsageWindow) UpstreamUsageAccountView {
		return UpstreamUsageAccountView{Available: true, Usage: &UpstreamUsageData{SevenDay: week, FiveHour: short}}
	}
	unknown := window(100, 1)
	unknown.ResetsAt = ""
	remaining := account(window(100, 1), nil)
	remaining.Usage.SevenDay.ResetsAt = "invalid"
	remaining.Usage.SevenDay.RemainingSeconds = 7200
	remaining.FetchedAt = now.Add(-time.Hour).Unix()
	missingUtilization := &UpstreamUsageWindow{}

	tests := []struct {
		name      string
		accounts  []UpstreamUsageAccountView
		exhausted bool
		hours     int
	}{
		{"no battery", nil, false, 0},
		{"weekly exhausted", []UpstreamUsageAccountView{account(window(100, 3), nil)}, true, 3},
		{"five hour exhausted", []UpstreamUsageAccountView{account(window(20, 24), window(100, 2))}, true, 2},
		{"both windows wait for later reset", []UpstreamUsageAccountView{account(window(100, 24), window(100, 2))}, true, 24},
		{"first account to recover", []UpstreamUsageAccountView{account(window(100, 24), window(100, 2)), account(window(100, 3), nil)}, true, 3},
		{"mixed exhausted windows", []UpstreamUsageAccountView{account(window(20, 24), window(100, 2)), account(window(100, 3), nil)}, true, 2},
		{"one account still usable", []UpstreamUsageAccountView{account(window(100, 3), nil), account(window(99, 3), nil)}, false, 0},
		{"unavailable snapshot", []UpstreamUsageAccountView{{Usage: &UpstreamUsageData{SevenDay: window(100, 3)}}}, false, 0},
		{"missing usage", []UpstreamUsageAccountView{{Available: true}}, false, 0},
		{"missing weekly usage", []UpstreamUsageAccountView{account(nil, window(100, 2))}, false, 0},
		{"missing utilization", []UpstreamUsageAccountView{account(missingUtilization, nil)}, false, 0},
		{"invalid utilization", []UpstreamUsageAccountView{account(window(101, 3), nil)}, false, 0},
		{"reset already passed", []UpstreamUsageAccountView{account(window(100, -1), nil)}, false, 0},
		{"exact reset boundary", []UpstreamUsageAccountView{account(window(100, 0), nil)}, false, 0},
		{"other exhausted window remains", []UpstreamUsageAccountView{account(window(100, -1), window(100, 2))}, true, 2},
		{"unknown reset", []UpstreamUsageAccountView{account(unknown, nil)}, true, 0},
		{"unknown account recovery", []UpstreamUsageAccountView{account(window(100, 3), nil), account(unknown, nil)}, true, 0},
		{"remaining seconds anchored to fetch", []UpstreamUsageAccountView{remaining}, true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reset, exhausted := exhaustedUpstreamUsageReset(tt.accounts, now)
			require.Equal(t, tt.exhausted, exhausted)
			if tt.hours == 0 {
				require.True(t, reset.IsZero())
			} else {
				require.True(t, now.Add(time.Duration(tt.hours)*time.Hour).Equal(reset))
			}
		})
	}
}

func TestUpstreamUsageExhaustedClientMessage(t *testing.T) {
	setupUpstreamUsageDB(t)
	reset := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	input := model.UpstreamUsageProviderInput{
		BaseURL: "https://example.com", AccessToken: "secret", RefreshToken: "secret", IntervalMinutes: 5,
		Accounts: []model.UpstreamUsageAccountInput{{AccountID: 1, Groups: []string{"exhausted"}}, {AccountID: 2, Groups: []string{"available"}}},
	}
	require.NoError(t, SaveUpstreamUsage(0, input))
	providers, err := model.ListUpstreamUsageProviders()
	require.NoError(t, err)
	for _, account := range providers[0].Accounts {
		used := float64(100)
		if account.AccountID == 2 {
			used = 50
		}
		usage, marshalErr := common.Marshal(UpstreamUsageData{SevenDay: &UpstreamUsageWindow{Utilization: &used, ResetsAt: reset.Format(time.RFC3339)}})
		require.NoError(t, marshalErr)
		require.NoError(t, model.SaveUpstreamUsageResult(account.ID, string(usage), time.Now().Unix(), ""))
	}
	message := "No available accounts: no available accounts"
	rawBody := `{"error":{"type":"api_error","message":"` + message + `"}}`
	want := "限额已用光，下次重置时间：" + reset.In(time.FixedZone("UTC+8", 8*3600)).Format("2006-01-02 15:04:05") + "（UTC+8）"
	for _, tt := range []struct {
		name, group, body string
		status            int
		mapping           string
		rewritten         bool
	}{
		{"matching error", "exhausted", rawBody, 503, "", true},
		{"plain text error", "exhausted", message, 503, "", true},
		{"mapped upstream status", "exhausted", rawBody, 503, `{"503":429}`, true},
		{"no battery", "unbound", rawBody, 503, "", false},
		{"available group", "available", rawBody, 503, "", false},
		{"no selected group", "", rawBody, 503, "", false},
		{"other upstream error", "exhausted", `{"error":{"message":"overloaded"}}`, 503, "", false},
		{"other upstream status", "exhausted", rawBody, 502, "", false},
		{"mapped to 503", "exhausted", rawBody, 502, `{"502":503}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := RelayErrorHandler(context.Background(), &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body))}, false)
			ResetStatusCode(apiErr, tt.mapping)
			originalMessage, originalStatus, originalCode := apiErr.Error(), apiErr.StatusCode, apiErr.GetErrorCode()
			SetUpstreamUsageExhaustedMessage(tt.group, apiErr)
			require.Equal(t, originalMessage, apiErr.Error(), "keep upstream error for logs and retry decisions")
			require.Equal(t, originalStatus, apiErr.StatusCode)
			require.Equal(t, originalCode, apiErr.GetErrorCode())
			require.Equal(t, tt.body, apiErr.ResponseBody)
			require.False(t, types.IsSkipRetryError(apiErr))
			if tt.rewritten {
				require.Equal(t, want, apiErr.ToOpenAIErrorForResponse().Message)
				require.Equal(t, want, apiErr.ToClaudeErrorForResponse().Message)
			} else {
				require.Empty(t, apiErr.ResponseMessage)
			}
		})
	}
	for _, updates := range []map[string]interface{}{
		{"last_error": "upstream HTTP 503"},
		{"last_error": "", "fetched_at": time.Now().Add(-time.Hour).Unix()},
	} {
		require.NoError(t, model.DB.Model(&model.UpstreamUsageAccount{}).Where("account_id = ?", 1).Updates(updates).Error)
		apiErr := RelayErrorHandler(context.Background(), &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(rawBody))}, false)
		SetUpstreamUsageExhaustedMessage("exhausted", apiErr)
		require.Empty(t, apiErr.ResponseMessage, "failed or stale snapshots must not claim exhaustion")
	}
}
