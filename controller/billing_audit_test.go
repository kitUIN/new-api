package controller_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingAuditAdminAuthorizationAndInputValidation(t *testing.T) {
	for _, role := range []int{0, common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		for _, test := range []struct{ method, path, body string }{
			{http.MethodGet, "/api/billing-audit?month=invalid", ""},
			{http.MethodGet, "/api/billing-audit/topups?month=2026-09&page=0", ""},
			{http.MethodGet, "/api/billing-audit/topups?month=2026-09&view=users&page=0", ""},
			{http.MethodGet, "/api/billing-audit/topups?month=2026-09&view=invalid", ""},
			{http.MethodPost, "/api/billing-audit/costs", `{"month":"2026-09","name":"x","amount":"0"}`},
			{http.MethodPut, "/api/billing-audit/costs/0", `{}`},
			{http.MethodPut, "/api/billing-audit/excluded-groups", `{"month":"invalid","excluded_groups":[]}`},
			{http.MethodPut, "/api/billing-audit/excluded-groups", `{"excluded_groups":[]}`},
			{http.MethodPut, "/api/billing-audit/excluded-groups", `{"month":"2026-09"}`},
			{http.MethodPut, "/api/billing-audit/excluded-groups", `{"month":"2026-09","excluded_groups":null}`},
			{http.MethodPut, "/api/billing-audit/excluded-groups", `{"month":"2026-09","excluded_groups":[1]}`},
			{http.MethodDelete, "/api/billing-audit/costs/no-id", `{}`},
		} {
			router := gin.New()
			router.Use(sessions.Sessions("billing-test", cookie.NewStore([]byte("billing-test-session-key-32-bytes"))))
			router.Use(func(c *gin.Context) {
				if role != 0 {
					session := sessions.Default(c)
					session.Set("username", "audit-admin")
					session.Set("role", role)
					session.Set("id", 1)
					session.Set("status", common.UserStatusEnabled)
				}
				c.Next()
			})
			group := router.Group("/api/billing-audit", middleware.AdminAuth())
			group.GET("", controller.GetBillingAudit)
			group.PUT("/excluded-groups", controller.SaveBillingAuditExcludedGroups)
			group.GET("/topups", controller.GetBillingAuditTopUps)
			group.POST("/costs", controller.SaveBillingAuditCost)
			group.PUT("/costs/:id", controller.SaveBillingAuditCost)
			group.DELETE("/costs/:id", controller.SaveBillingAuditCost)
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("New-Api-User", "1")
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var payload struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			require.False(t, payload.Success)
			if role >= common.RoleAdminUser {
				require.Equal(t, http.StatusBadRequest, response.Code)
			} else {
				require.NotEqual(t, http.StatusBadRequest, response.Code)
			}
		}
	}
}

func TestBillingAuditSaveAndClearExclusions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.BillingAuditPreference{}))
	router := gin.New()
	router.PUT("/excluded-groups", func(c *gin.Context) { c.Set("id", 42) }, controller.SaveBillingAuditExcludedGroups)
	for _, groups := range [][]string{{"internal", ""}, {}} {
		body, err := common.Marshal(map[string]interface{}{"month": "2026-09", "excluded_groups": groups})
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodPut, "/excluded-groups", strings.NewReader(string(body)))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.JSONEq(t, `{"success":true}`, response.Body.String())
		saved, err := model.GetBillingAuditExcludedGroups("2026-09")
		require.NoError(t, err)
		require.Equal(t, groups, saved)
		var preference model.BillingAuditPreference
		require.NoError(t, db.First(&preference).Error)
		require.Equal(t, 42, preference.UpdatedBy)
	}
}
