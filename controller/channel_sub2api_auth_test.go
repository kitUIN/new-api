package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestRecoverSub2APIProviderQueryFallsBackToLogin(t *testing.T) {
	tests := []struct {
		name                string
		refreshUnauthorized bool
	}{
		{name: "refreshed access token remains unauthorized"},
		{name: "refresh endpoint returns unauthorized", refreshUnauthorized: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			refreshCalls := 0
			loginCalls := 0
			writeTokens := func(w http.ResponseWriter, accessToken string, refreshToken string) {
				body, err := common.Marshal(map[string]interface{}{
					"code":    0,
					"message": "success",
					"data": map[string]interface{}{
						"access_token":  accessToken,
						"refresh_token": refreshToken,
						"expires_in":    86400,
						"token_type":    "Bearer",
					},
				})
				if err != nil {
					t.Errorf("marshal response: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/auth/refresh":
					refreshCalls++
					var payload map[string]string
					if err := common.DecodeJson(r.Body, &payload); err != nil {
						t.Errorf("decode refresh payload: %v", err)
					}
					if payload["refresh_token"] != "old_refresh" {
						t.Errorf("unexpected refresh token: %q", payload["refresh_token"])
					}
					if test.refreshUnauthorized {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					writeTokens(w, "refreshed_access", "refreshed_refresh")
				case "/api/v1/auth/login":
					loginCalls++
					var payload map[string]string
					if err := common.DecodeJson(r.Body, &payload); err != nil {
						t.Errorf("decode login payload: %v", err)
					}
					if payload["email"] != "user@example.com" {
						t.Errorf("unexpected login email: %q", payload["email"])
					}
					if payload["password"] != " secret " {
						t.Errorf("unexpected login password")
					}
					writeTokens(w, "login_access", "login_refresh")
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			baseURL := server.URL
			channel := &model.Channel{BaseURL: &baseURL}
			provider := &model.ChannelProvider{}
			settings := dto.ChannelProviderSettings{
				Sub2APIEmail:    " user@example.com ",
				Sub2APIPassword: " secret ",
				BalanceQuery: dto.BalanceQuery{
					AccessToken:  "old_balance_access",
					RefreshToken: "old_refresh",
				},
				GroupQuery: dto.GroupQuery{
					AccessToken:  "old_group_access",
					RefreshToken: "old_refresh",
				},
			}
			retriedTokens := make([]sub2APIAuthTokens, 0, 2)
			retry := func(tokens sub2APIAuthTokens) ([]byte, error) {
				retriedTokens = append(retriedTokens, tokens)
				if !test.refreshUnauthorized && len(retriedTokens) == 1 {
					return nil, &responseStatusError{StatusCode: http.StatusUnauthorized}
				}
				return []byte(`{"ok":true}`), nil
			}

			body, updated, err := recoverSub2APIProviderQuery(
				channel,
				provider,
				settings,
				balanceQueryTemplateSub2API,
				"old_refresh",
				&responseStatusError{StatusCode: http.StatusUnauthorized},
				retry,
			)
			if err != nil {
				t.Fatalf("recover query: %v", err)
			}
			if string(body) != `{"ok":true}` {
				t.Fatalf("unexpected query body: %s", body)
			}
			if refreshCalls != 1 || loginCalls != 1 {
				t.Fatalf("expected one refresh and one login, got refresh=%d login=%d", refreshCalls, loginCalls)
			}
			if updated.BalanceQuery.AccessToken != "login_access" || updated.BalanceQuery.RefreshToken != "login_refresh" {
				t.Fatalf("balance query tokens were not saved: %+v", updated.BalanceQuery)
			}
			if updated.GroupQuery.AccessToken != "login_access" || updated.GroupQuery.RefreshToken != "login_refresh" {
				t.Fatalf("group query tokens were not saved: %+v", updated.GroupQuery)
			}
			persisted := provider.GetOtherSettings()
			if persisted.Sub2APIEmail != settings.Sub2APIEmail || persisted.Sub2APIPassword != settings.Sub2APIPassword {
				t.Fatalf("sub2api credentials were not preserved: %+v", persisted)
			}
			if persisted.BalanceQuery.AccessToken != "login_access" || persisted.GroupQuery.RefreshToken != "login_refresh" {
				t.Fatalf("provider tokens were not persisted in settings: %+v", persisted)
			}
		})
	}
}

func TestRecoverSub2APIProviderQueryIgnoresOtherTemplates(t *testing.T) {
	queryErr := &responseStatusError{StatusCode: http.StatusUnauthorized}
	retryCalls := 0
	provider := &model.ChannelProvider{}

	body, updated, err := recoverSub2APIProviderQuery(
		&model.Channel{},
		provider,
		dto.ChannelProviderSettings{
			Sub2APIEmail:    "user@example.com",
			Sub2APIPassword: "password",
		},
		balanceQueryTemplateNewAPI,
		"old_refresh",
		queryErr,
		func(tokens sub2APIAuthTokens) ([]byte, error) {
			retryCalls++
			return nil, nil
		},
	)
	if err != queryErr {
		t.Fatalf("expected original query error, got %v", err)
	}
	if body != nil || retryCalls != 0 {
		t.Fatalf("non-sub2api query should not retry, body=%q retries=%d", body, retryCalls)
	}
	if updated.Sub2APIEmail != "user@example.com" || provider.Settings != "" {
		t.Fatalf("non-sub2api query should not mutate provider settings: %+v", updated)
	}
}
