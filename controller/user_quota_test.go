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
)

func TestManageUserQuotaModes(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousSQLite, previousMySQL, previousPostgreSQL, previousRedis := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL, common.RedisEnabled
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL, common.RedisEnabled = previousSQLite, previousMySQL, previousPostgreSQL, previousRedis
	})
	db := setupUserAccessTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
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
