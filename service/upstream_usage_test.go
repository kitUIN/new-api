package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpstreamUsageDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.UpstreamUsageProvider{}, &model.UpstreamUsageAccount{}))
}

func TestUpstreamUsageRefreshPersistenceAndFailure(t *testing.T) {
	setupUpstreamUsageDB(t)
	refreshes, requests := 0, 0
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/refresh" {
			refreshes++
			require.Equal(t, http.MethodPost, r.Method)
			var body map[string]string
			require.NoError(t, common.DecodeJson(r.Body, &body))
			require.Equal(t, "refresh-secret", body["refresh_token"])
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"new-access-secret","refresh_token":"new-refresh-secret"}}`))
			return
		}
		requests++
		require.Equal(t, "Asia/Shanghai", r.URL.Query().Get("timezone"))
		require.Contains(t, []string{"/api/v1/admin/accounts/1/usage", "/api/v1/admin/accounts/2/usage"}, r.URL.Path)
		if fail {
			w.WriteHeader(503)
			_, _ = w.Write([]byte("sensitive upstream body"))
			return
		}
		if r.Header.Get("Authorization") != "Bearer new-access-secret" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"updated_at":"2026-09-28T11:09:17+08:00","five_hour":{"utilization":0,"remaining_seconds":0},"seven_day":{"utilization":30,"window_stats":{"requests":3998,"tokens":466280056,"cost":451.41,"standard_cost":451.41,"user_cost":63.19}}}}`))
	}))
	defer server.Close()
	input := model.UpstreamUsageProviderInput{BaseURL: server.URL, AccessToken: "old-access-secret", RefreshToken: "refresh-secret", IntervalMinutes: 5, Accounts: []model.UpstreamUsageAccountInput{{AccountID: 1, Groups: []string{"group-a", "group-a"}}, {AccountID: 2, Groups: []string{"group-a", "group-b"}}}}
	require.NoError(t, SaveUpstreamUsage(0, input))
	providers, err := model.ListUpstreamUsageProviders()
	require.NoError(t, err)
	id := providers[0].ID
	require.NoError(t, PollUpstreamUsage(context.Background(), id, true))
	require.Equal(t, 1, refreshes)
	require.Equal(t, 3, requests)
	require.NoError(t, PollUpstreamUsage(context.Background(), id, true))
	require.Equal(t, 3, requests, "not yet due")
	saved, err := model.GetUpstreamUsageProvider(id)
	require.NoError(t, err)
	require.Equal(t, "new-refresh-secret", saved.RefreshToken)
	views, err := ListUpstreamUsage()
	require.NoError(t, err)
	require.True(t, views[0].Accounts[0].Available)
	require.Equal(t, float64(0), *views[0].Accounts[0].Usage.FiveHour.Utilization)
	require.Equal(t, []string{"group-a"}, views[0].Accounts[0].Groups)
	encoded, err := common.Marshal(views)
	require.NoError(t, err)
	for _, secret := range []string{"access_token", "refresh_token", "secret", "user_cost"} {
		require.NotContains(t, string(encoded), secret)
	}
	// Blank credentials preserve rotated tokens and retained accounts keep their snapshot.
	input.AccessToken, input.RefreshToken = "", ""
	input.Accounts = input.Accounts[:1]
	require.NoError(t, SaveUpstreamUsage(id, input))
	saved, err = model.GetUpstreamUsageProvider(id)
	require.NoError(t, err)
	require.Equal(t, "new-access-secret", saved.AccessToken)
	require.Len(t, saved.Accounts, 1)
	require.NotEmpty(t, saved.Accounts[0].UsageJSON)
	fail = true
	require.NoError(t, PollUpstreamUsage(context.Background(), id, false))
	views, err = ListUpstreamUsage()
	require.NoError(t, err)
	require.False(t, views[0].Accounts[0].Available)
	require.Equal(t, "upstream HTTP 503", views[0].Accounts[0].LastError)
	require.NotNil(t, views[0].Accounts[0].Usage, "retain last successful snapshot")
	require.NoError(t, model.DB.Model(&model.UpstreamUsageAccount{}).Where("provider_id = ?", id).Updates(map[string]interface{}{"last_error": "", "fetched_at": time.Now().Add(-time.Hour).Unix()}).Error)
	views, err = ListUpstreamUsage()
	require.NoError(t, err)
	require.False(t, views[0].Accounts[0].Available, "expired snapshots are unavailable")
	require.NoError(t, DeleteUpstreamUsage(id))
	var count int64
	require.NoError(t, model.DB.Model(&model.UpstreamUsageAccount{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpstreamUsageInvalidResponses(t *testing.T) {
	for _, body := range []string{`{}`, `{"code":1,"data":{}}`, `{"code":0,"data":null}`, `{"code":0,"data":{"seven_day":{}}}`, `{"code":0,"data":{"seven_day":{"utilization":101}}}`, `{"code":0,"data":{"seven_day":{"utilization":-1}}}`, `not json`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			canRefresh := false
			_, err := fetchUpstreamAccountUsage(context.Background(), &model.UpstreamUsageProvider{BaseURL: server.URL}, 1, &canRefresh)
			require.Error(t, err)
		})
	}
}

func TestUpstreamUsageValidationAndBaseChange(t *testing.T) {
	setupUpstreamUsageDB(t)
	input := model.UpstreamUsageProviderInput{BaseURL: "https://example.com", AccessToken: "access", RefreshToken: "refresh", IntervalMinutes: 1, Accounts: []model.UpstreamUsageAccountInput{{AccountID: 1}}}
	for _, url := range []string{"file:///tmp/foo", "https://user:pass@example.com", "https://example.com?token=secret", "ftp://example.com"} {
		invalid := input
		invalid.BaseURL = url
		require.Error(t, invalid.Validate())
	}
	invalid := input
	invalid.IntervalMinutes = 0
	require.Error(t, invalid.Validate())
	invalid = input
	invalid.Accounts = []model.UpstreamUsageAccountInput{{AccountID: 1}, {AccountID: 1}}
	require.Error(t, invalid.Validate())
	require.NoError(t, SaveUpstreamUsage(0, input))
	providers, err := model.ListUpstreamUsageProviders()
	require.NoError(t, err)
	id := providers[0].ID
	require.NoError(t, model.SaveUpstreamUsageResult(providers[0].Accounts[0].ID, `{"seven_day":{"utilization":0}}`, time.Now().Unix(), ""))
	input.BaseURL = "https://new.example.com"
	input.AccessToken = ""
	require.Error(t, SaveUpstreamUsage(id, input), "must not reuse credentials for another host")
	input.AccessToken = "new"
	require.NoError(t, SaveUpstreamUsage(id, input))
	p, err := model.GetUpstreamUsageProvider(id)
	require.NoError(t, err)
	require.Empty(t, p.Accounts[0].UsageJSON)
	require.Zero(t, p.Accounts[0].FetchedAt)
	input.Accounts = nil
	require.NoError(t, SaveUpstreamUsage(id, input))
	p, err = model.GetUpstreamUsageProvider(id)
	require.NoError(t, err)
	require.Empty(t, p.Accounts)
}

func TestUpstreamUsageDoesNotFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://example.com")
		w.WriteHeader(302)
	}))
	defer server.Close()
	canRefresh := false
	_, err := fetchUpstreamAccountUsage(context.Background(), &model.UpstreamUsageProvider{BaseURL: server.URL}, 1, &canRefresh)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "302"))
}
