package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetUserModelsByGroup(t *testing.T) {
	require.NoError(t, i18n.Init())
	oldDB, oldRedis := model.DB, common.RedisEnabled
	oldUsable := setting.UserUsableGroups2JSONString()
	oldAuto := setting.AutoGroups2JsonString()
	oldRules := setting.RuleAutoGroupsEnabled()
	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldCombinations := ratio_setting.GroupCombinations2JSONString()
	t.Cleanup(func() {
		model.DB, common.RedisEnabled = oldDB, oldRedis
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAuto))
		setting.SetRuleAutoGroupsEnabled(oldRules)
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(oldCombinations))
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	model.DB, common.RedisEnabled = db, false
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}))
	user := model.User{Username: "playground-model-test", Group: "user"}
	require.NoError(t, db.Create(&user).Error)
	for _, group := range []string{"codex-a", "other", "private"} {
		channel := model.Channel{Key: "test", Group: group, Status: common.ChannelStatusEnabled}
		require.NoError(t, db.Create(&channel).Error)
		name := map[string]string{"codex-a": "model-a", "other": "model-b", "private": "secret-model"}[group]
		require.NoError(t, db.Create(&[]model.Ability{
			{Group: group, Model: name, ChannelId: channel.Id, Enabled: true},
			{Group: group, Model: "shared", ChannelId: channel.Id, Enabled: true},
			{Group: group, Model: "disabled-model", ChannelId: channel.Id, Enabled: false},
		}).Error)
	}
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"codex-a":"A","other":"B","combo":"Combo","auto":"Auto"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["codex-a","private"]`))
	setting.SetRuleAutoGroupsEnabled(true)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"codex-a":0.05,"other":1,"private":1,"combo":1}`))
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(`{"combo":[{"group":"codex-a","models":["model-a"]},{"group":"other","models":["model-b"]}]}`))

	for _, tc := range []struct {
		name, group string
		status      int
		models      []string
	}{
		{"normal group", "codex-a", http.StatusOK, []string{"model-a", "shared"}},
		{"other group", "other", http.StatusOK, []string{"model-b", "shared"}},
		{"empty group", "user", http.StatusOK, []string{}},
		{"unavailable group", "private", http.StatusForbidden, nil},
		{"automatic group respects access", "auto", http.StatusOK, []string{"model-a", "shared"}},
		{"rule automatic group", service.RuleAutoGroupCodexLow, http.StatusOK, []string{"model-a", "shared"}},
		{"combination filters models", "combo", http.StatusOK, []string{"model-a", "model-b"}},
		{"no filter preserves union", "", http.StatusOK, []string{"model-a", "model-b", "shared"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("id", user.Id)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/user/models?group="+url.QueryEscape(tc.group), nil)
			GetUserModels(c)
			require.Equal(t, tc.status, recorder.Code)
			var response struct {
				Success bool     `json:"success"`
				Data    []string `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			if tc.status == http.StatusOK {
				require.True(t, response.Success)
				require.NotNil(t, response.Data)
				require.ElementsMatch(t, tc.models, response.Data)
			} else {
				require.False(t, response.Success)
			}
		})
	}
}
