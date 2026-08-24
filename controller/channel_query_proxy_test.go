package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestExecuteProviderConfiguredBalanceUsesQueryProxy(t *testing.T) {
	originalDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ChannelProvider{}))
	model.DB = db
	service.ResetProxyClientCache()
	t.Cleanup(func() {
		service.ResetProxyClientCache()
		model.DB = originalDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	proxyCalls := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalls++
		if r.URL.Host != "upstream.example" {
			t.Errorf("proxy request host = %q, want upstream.example", r.URL.Host)
		}
		if r.URL.Path != "/balance" {
			t.Errorf("proxy request path = %q, want /balance", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"balance":12.5}}`))
	}))
	defer proxyServer.Close()

	settings := dto.ChannelProviderSettings{
		QueryProxy: "  " + proxyServer.URL + "  ",
		BalanceQuery: dto.BalanceQuery{
			Enabled: true,
			Request: dto.BalanceQueryRequestConfig{
				URL:    "http://upstream.example/balance",
				Method: http.MethodGet,
			},
			Extractor: dto.BalanceQueryExtractorConfig{
				RemainingPath: "data.balance",
				Divisor:       1,
				SuccessPath:   "success",
				SuccessValue:  "true",
			},
		},
	}
	provider := &model.ChannelProvider{
		Name:    "provider-query-proxy",
		BaseURL: "http://upstream.example",
		Status:  common.ChannelStatusEnabled,
	}
	provider.SetOtherSettings(settings)
	require.NoError(t, db.Create(provider).Error)

	baseURL := provider.BaseURL
	channel := &model.Channel{Id: 77, ProviderID: provider.Id, Key: "sk-test", BaseURL: &baseURL}
	channel.SetSetting(dto.ChannelSettings{Proxy: "http://127.0.0.1:1"})

	balance, result, configured, err := executeProviderConfiguredBalance(
		provider,
		settings,
		providerBalanceQueryExecution{Provider: provider, Channel: channel, Settings: settings},
	)

	require.NoError(t, err)
	require.True(t, configured)
	require.NotNil(t, result)
	require.True(t, result.IsValid)
	require.Equal(t, 12.5, balance)
	require.Equal(t, 1, proxyCalls)
	require.Equal(t, settings.QueryProxy, provider.GetOtherSettings().QueryProxy)
}
