package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserQuotaTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousSQLite, previousMySQL, previousPostgreSQL, previousRedis := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL, common.RedisEnabled
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL, common.RedisEnabled = previousSQLite, previousMySQL, previousPostgreSQL, previousRedis
	})
	db := setupUserAccessTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	return db
}

func TestManageUserQuotaModes(t *testing.T) {
	db := setupUserQuotaTest(t)
	user := model.User{Id: 100, Username: "quota-user", Role: common.RoleCommonUser, Quota: 1000}
	require.NoError(t, db.Create(&user).Error)

	for _, test := range []struct {
		name    string
		mode    string
		value   int
		role    int
		quota   int
		logType int
		success bool
	}{
		{"recharge", "recharge", 200, common.RoleAdminUser, 1200, model.LogTypeTopup, true},
		{"refund", "refund", 200, common.RoleAdminUser, 800, model.LogTypeRefund, true},
		{"add unchanged", "add", 200, common.RoleAdminUser, 1200, model.LogTypeManage, true},
		{"subtract unchanged", "subtract", 200, common.RoleAdminUser, 800, model.LogTypeManage, true},
		{"override unchanged", "override", 200, common.RoleAdminUser, 200, model.LogTypeManage, true},
		{"zero recharge", "recharge", 0, common.RoleAdminUser, 1000, 0, false},
		{"negative recharge", "recharge", -200, common.RoleAdminUser, 1000, 0, false},
		{"zero refund", "refund", 0, common.RoleAdminUser, 1000, 0, false},
		{"negative refund", "refund", -200, common.RoleAdminUser, 1000, 0, false},
		{"subtract below zero", "subtract", 1001, common.RoleAdminUser, 1000, 0, false},
		{"refund below zero", "refund", 1001, common.RoleAdminUser, 1000, 0, false},
		{"negative override", "override", -1, common.RoleAdminUser, 1000, 0, false},
		{"unknown mode", "unknown", 200, common.RoleAdminUser, 1000, 0, false},
		{"recharge permission", "recharge", 200, common.RoleCommonUser, 1000, 0, false},
		{"refund permission", "refund", 200, common.RoleCommonUser, 1000, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, db.Model(&user).Update("quota", 1000).Error)
			require.NoError(t, db.Where("user_id = ?", user.Id).Delete(&model.Log{}).Error)
			body, err := common.Marshal(ManageRequest{Id: user.Id, Action: "add_quota", Mode: test.mode, Value: test.value})
			require.NoError(t, err)
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", bytes.NewReader(body))
			ctx.Set("id", 1)
			ctx.Set("username", "quota-admin")
			ctx.Set("role", test.role)
			ManageUser(ctx)
			var result struct{ Success bool }
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			require.Equal(t, test.success, result.Success)
			var updated model.User
			require.NoError(t, db.First(&updated, user.Id).Error)
			require.Equal(t, test.quota, updated.Quota)
			var logs []model.Log
			require.NoError(t, db.Where("user_id = ?", user.Id).Find(&logs).Error)
			if !test.success {
				require.Empty(t, logs)
				return
			}
			require.Len(t, logs, 1)
			require.Equal(t, test.logType, logs[0].Type)
			require.Contains(t, logs[0].Other, `"admin_username":"quota-admin"`)
			require.Contains(t, logs[0].Other, `"admin_id":1`)
		})
	}
}

func TestUserNegativeBalancePermission(t *testing.T) {
	db := setupUserQuotaTest(t)
	user := model.User{Id: 100, Username: "wallet-user", Role: common.RoleCommonUser}
	require.NoError(t, db.Create(&user).Error)

	edit := func(handler gin.HandlerFunc, role int, flag *bool) bool {
		t.Helper()
		data := map[string]any{"id": user.Id, "username": user.Username}
		if flag != nil {
			data["allow_negative_balance"] = *flag
		}
		body, err := common.Marshal(data)
		require.NoError(t, err)
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
		ctx.Set("role", role)
		ctx.Set("id", user.Id)
		handler(ctx)
		var result struct{ Success bool }
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
		return result.Success
	}
	on, off := true, false
	require.True(t, edit(UpdateSelf, common.RoleCommonUser, &on))
	require.False(t, edit(UpdateUser, common.RoleCommonUser, &on))
	require.NoError(t, db.First(&user, user.Id).Error)
	require.False(t, user.AllowNegativeBalance)

	require.True(t, edit(UpdateUser, common.RoleAdminUser, &on))
	require.True(t, edit(UpdateUser, common.RoleAdminUser, nil))
	require.NoError(t, db.First(&user, user.Id).Error)
	require.True(t, user.AllowNegativeBalance)
	_, err := model.AdjustQuotaByAdmin(user.Id, -10, false)
	require.NoError(t, err)
	_, err = model.AdjustQuotaByAdmin(user.Id, -20, true)
	require.NoError(t, err)

	require.True(t, edit(UpdateUser, common.RoleAdminUser, &off))
	require.NoError(t, db.First(&user, user.Id).Error)
	require.False(t, user.AllowNegativeBalance)
	require.Equal(t, -20, user.Quota)
	_, err = model.AdjustQuotaByAdmin(user.Id, 5, false)
	require.NoError(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, -15, user.Quota)
}
