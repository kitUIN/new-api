package router

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpstreamUsageAdminRoutesAndPublicProjection(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.UpstreamUsageProvider{}, &model.UpstreamUsageAccount{}))
	oldRateLimit := common.GlobalApiRateLimitEnable
	common.GlobalApiRateLimitEnable = false
	t.Cleanup(func() { common.GlobalApiRateLimitEnable = oldRateLimit })
	engine := gin.New()
	engine.Use(sessions.Sessions("upstream-usage-test", cookie.NewStore([]byte("usage-test-only"))))
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
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response.Body.String()
	}
	for _, route := range []struct{ method, path string }{{"GET", "/api/upstream-usage"}, {"POST", "/api/upstream-usage"}, {"PUT", "/api/upstream-usage/1"}, {"DELETE", "/api/upstream-usage/1"}, {"POST", "/api/upstream-usage/1/refresh"}} {
		for _, role := range []int{0, common.RoleCommonUser} {
			require.Contains(t, request(route.method, route.path, `{}`, role), `"success":false`)
		}
	}
	body := `{"base_url":"https://private-upstream.example","access_token":"access-secret","refresh_token":"refresh-secret","interval_minutes":5,"accounts":[{"account_id":11,"groups":["shared"]},{"account_id":22,"groups":[]}]}`
	require.Contains(t, request("POST", "/api/upstream-usage", body, common.RoleAdminUser), `"success":true`)
	adminView := request("GET", "/api/upstream-usage", "", common.RoleAdminUser)
	require.Contains(t, adminView, "private-upstream.example")
	require.NotContains(t, adminView, "secret")
	publicView := request("GET", "/api/perf-metrics/upstream-usage", "", 0)
	require.Contains(t, publicView, `"account_id":11`)
	require.NotContains(t, publicView, `"account_id":22`)
	for _, value := range []string{"private-upstream", "access_token", "refresh_token", "secret", "user_cost"} {
		require.NotContains(t, publicView, value)
	}
}
