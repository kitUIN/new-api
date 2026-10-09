package controller_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
			{http.MethodGet, "/api/billing-audit/topups?month=2026-09&view=days&page=0", ""},
			{http.MethodGet, "/api/billing-audit/topups?month=invalid&view=days", ""},
			{http.MethodGet, "/api/billing-audit/topups?month=2026-09&view=days&page_size=101", ""},
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

func TestBillingAuditDailyTopUps(test *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(test, err)
	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(test, err)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, logDB
	test.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	for _, database := range []*gorm.DB{db, logDB} {
		sqlDB, err := database.DB()
		require.NoError(test, err)
		sqlDB.SetMaxOpenConns(1)
		test.Cleanup(func() { _ = sqlDB.Close() })
	}
	require.NoError(test, db.AutoMigrate(&model.TopUp{}))
	require.NoError(test, logDB.AutoMigrate(&model.Log{}))
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600)).Unix()
	require.NoError(test, db.Create(&[]model.TopUp{
		{TradeNo: "normal", CompleteTime: start, Money: 100, Status: common.TopUpStatusSuccess},
		{TradeNo: "xzn", PaymentMethod: model.PaymentMethodXznPay, CompleteTime: start - 86400, Money: 100, Status: common.TopUpStatusSuccess},
	}).Error)
	require.NoError(test, logDB.Create(&model.Log{Type: model.LogTypeRefund, CreatedAt: start + 86400, Content: "管理员退款扣减用户额度 ＄5.000000 额度"}).Error)
	router := gin.New()
	router.GET("/topups", controller.GetBillingAuditTopUps)
	for _, expected := range []struct {
		page, date, received, refunded, net string
		count                               int64
	}{
		{"1", "2026-10-02", "0", "5", "-5", 1},
		{"2", "2026-10-01", "198", "0", "198", 2},
		{"3", "", "", "", "", 0},
	} {
		request := httptest.NewRequest(http.MethodGet, "/topups?month=2026-10&view=days&page_size=1&page="+expected.page, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(test, http.StatusOK, response.Code)
		var payload struct {
			Success bool `json:"success"`
			Data    struct {
				Items []model.BillingDailyTopUpRow `json:"items"`
				Total int64                        `json:"total"`
			} `json:"data"`
		}
		require.NoError(test, common.Unmarshal(response.Body.Bytes(), &payload))
		require.True(test, payload.Success)
		require.Equal(test, int64(2), payload.Data.Total)
		if expected.date == "" {
			require.NotNil(test, payload.Data.Items)
			require.Empty(test, payload.Data.Items)
			continue
		}
		require.Len(test, payload.Data.Items, 1)
		row := payload.Data.Items[0]
		require.Equal(test, expected.date, row.Date)
		require.Equal(test, expected.received, row.Received.String())
		require.Equal(test, expected.refunded, row.Refunded.String())
		require.Equal(test, expected.net, row.NetRecharge.String())
		require.Equal(test, expected.count, row.RecordCount)
	}
}
