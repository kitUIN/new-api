package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestServiceProbeSettingsRootAccessAndPersistence(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}))
	oldOptions := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() { common.OptionMap = oldOptions })
	oldLimit := common.GlobalApiRateLimitEnable
	common.GlobalApiRateLimitEnable = false
	t.Cleanup(func() { common.GlobalApiRateLimitEnable = oldLimit })
	key := strings.Repeat("new-key-", 8)
	hive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+key, r.Header.Get("Authorization"))
		fmt.Fprint(w, `{"bees":[]}`)
	}))
	defer hive.Close()
	t.Setenv("GREATMAO_HIVE_URL", "https://legacy.example")
	t.Setenv("GREATMAO_HIVE_READ_API_KEY", strings.Repeat("legacy-key-", 8))
	engine := gin.New()
	engine.Use(sessions.Sessions("probe-settings-test", cookie.NewStore([]byte("probe-settings-test-only"))))
	engine.Use(func(c *gin.Context) {
		role, _ := strconv.Atoi(c.GetHeader("Test-Role"))
		if role > 0 {
			session := sessions.Default(c)
			session.Set("id", 1)
			session.Set("username", "tester")
			session.Set("role", role)
			session.Set("status", common.UserStatusEnabled)
		}
		c.Next()
	})
	SetApiRouter(engine)
	request := func(method, path, body string, role int) string {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Test-Role", strconv.Itoa(role))
		req.Header.Set("New-Api-User", "1")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response.Body.String()
	}
	const path = "/api/option/service-probes"
	body := fmt.Sprintf(`{"enabled":true,"url":%q,"read_api_key":%q}`, hive.URL, key)
	for _, role := range []int{0, common.RoleCommonUser, common.RoleAdminUser} {
		for _, method := range []string{"GET", "PUT"} {
			result := request(method, path, body, role)
			require.Contains(t, result, `"success":false`)
			require.NotContains(t, result, "legacy.example")
		}
	}
	legacy := request("GET", path, "", common.RoleRootUser)
	require.Contains(t, legacy, "legacy.example")
	require.Contains(t, legacy, `"key_configured":true`)
	require.NotContains(t, legacy, "legacy-key")
	result := request("PUT", path, body, common.RoleRootUser)
	require.Contains(t, result, `"success":true`)
	require.NotContains(t, result, key)
	_, err := service.ReadServiceProbe(context.Background(), "quotas", "", "")
	require.NoError(t, err, "saved configuration should be used without restart")
	var saved model.Option
	require.NoError(t, model.DB.Where(&model.Option{Key: model.ServiceProbeConfigOption}).First(&saved).Error)
	require.Contains(t, saved.Value, key)
	// Simulate loading the persisted row into a new instance's option map.
	common.OptionMap = map[string]string{saved.Key: saved.Value}
	result = request("GET", path, "", common.RoleRootUser)
	require.Contains(t, result, hive.URL)
	require.NotContains(t, result, key)
	result = request("GET", "/api/option/", "", common.RoleRootUser)
	require.NotContains(t, result, key)
	require.NotContains(t, result, model.ServiceProbeConfigOption)
	blankKey := fmt.Sprintf(`{"enabled":true,"url":%q,"read_api_key":""}`, hive.URL)
	require.Contains(t, request("PUT", path, blankKey, common.RoleRootUser), `"success":true`)
	_, err = service.ReadServiceProbe(context.Background(), "quotas", "", "")
	require.NoError(t, err)
	for _, invalid := range []string{`{"url":"https://missing-enabled.example"}`, `{"enabled":true,"url":"file:///etc/passwd"}`, `{"enabled":true,"url":"https://user:pass@example.com"}`, `{"enabled":true,"url":"https://example.com/?key=secret"}`, `{"enabled":true,"url":"https://example.com","read_api_key":"short"}`} {
		require.Contains(t, request("PUT", path, invalid, common.RoleRootUser), `"success":false`)
	}
	view, err := service.GetServiceProbeConfigView()
	require.NoError(t, err)
	require.Equal(t, hive.URL, view.URL)
	require.Error(t, model.UpdateOption(" serviceprobeconfigsecret ", "{}"))
	require.Contains(t, request("PUT", path, `{"enabled":false,"url":""}`, common.RoleRootUser), `"success":true`)
	_, err = service.ReadServiceProbe(context.Background(), "quotas", "", "")
	require.ErrorIs(t, err, service.ErrServiceProbeNotConfigured, "disabling must override environment defaults")
	require.NoError(t, model.DB.Migrator().DropTable(&model.Option{}))
	require.Contains(t, request("PUT", path, body, common.RoleRootUser), `"success":false`)
	view, err = service.GetServiceProbeConfigView()
	require.NoError(t, err)
	require.False(t, view.Enabled, "failed persistence must not update live configuration")
}

func TestServiceProbeAdminAccess(t *testing.T) {
	hits := 0
	hive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		fmt.Fprint(w, `{"bees":[{"id":"bee","ip":"192.0.2.10"}]}`)
	}))
	defer hive.Close()
	t.Setenv("GREATMAO_HIVE_URL", hive.URL)
	t.Setenv("GREATMAO_HIVE_READ_API_KEY", "test-secret")
	oldLimit := common.GlobalApiRateLimitEnable
	common.GlobalApiRateLimitEnable = false
	t.Cleanup(func() { common.GlobalApiRateLimitEnable = oldLimit })
	engine := gin.New()
	engine.Use(sessions.Sessions("probe-test", cookie.NewStore([]byte("probe-test-only"))))
	engine.Use(func(c *gin.Context) {
		role, _ := strconv.Atoi(c.GetHeader("Test-Role"))
		if role > 0 {
			session := sessions.Default(c)
			session.Set("id", 1)
			session.Set("username", "tester")
			session.Set("role", role)
			session.Set("status", common.UserStatusEnabled)
		}
		c.Next()
	})
	SetApiRouter(engine)
	for _, path := range []string{"/api/service-probes", "/api/service-probes/bee/current", "/api/service-probes/bee/history"} {
		for _, role := range []int{0, common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
			before := hits
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Test-Role", strconv.Itoa(role))
			req.Header.Set("New-Api-User", "1")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			body := response.Body.String()
			if role < common.RoleAdminUser {
				require.Contains(t, body, `"success":false`)
				require.NotContains(t, body, "192.0.2.10")
				require.Equal(t, before, hits)
			} else {
				require.Contains(t, body, `"success":true`)
				require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			}
			require.NotContains(t, body, "test-secret")
		}
	}
}
