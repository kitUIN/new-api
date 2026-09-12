package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelaySessionAPIPermissions(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.RelaySession{}, &model.Log{}))
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create([]model.RelaySession{
		{ID: "owner-session", UserID: 1, SessionKey: "owner-key", LastSeenAt: now, OverrideGroup: "B"},
		{ID: "other-session", UserID: 2, SessionKey: "other-key", LastSeenAt: now},
	}).Error)
	engine := gin.New()
	engine.Use(sessions.Sessions("test", cookie.NewStore([]byte("session-test-only"))))
	engine.Use(func(c *gin.Context) {
		role, _ := strconv.Atoi(c.GetHeader("Test-Role"))
		if role > 0 {
			s := sessions.Default(c)
			s.Set("id", 1)
			s.Set("username", "owner")
			s.Set("role", role)
			s.Set("status", common.UserStatusEnabled)
		}
		c.Next()
	})
	SetApiRouter(engine)
	request := func(method, path, body string, role int) map[string]interface{} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Test-Role", strconv.Itoa(role))
		req.Header.Set("New-Api-User", "1")
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, req)
		var response map[string]interface{}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response), recorder.Body.String())
		return response
	}
	self := request("GET", "/api/log/self/sessions?user_id=2", "", common.RoleCommonUser)
	require.Equal(t, true, self["success"])
	items := self["data"].(map[string]interface{})["items"].([]interface{})
	require.Len(t, items, 1)
	require.Equal(t, "owner-key", items[0].(map[string]interface{})["session_key"])
	for _, path := range []string{"/api/log/sessions", "/api/log/sessions/other-session/groups"} {
		require.Equal(t, false, request("GET", path, "", common.RoleCommonUser)["success"])
	}
	require.Equal(t, false, request("PUT", "/api/log/sessions/owner-session/group", `{"group":""}`, common.RoleCommonUser)["success"])
	stored, err := model.GetActiveRelaySession("owner-session")
	require.NoError(t, err)
	require.Equal(t, "B", stored.OverrideGroup)
	all := request("GET", "/api/log/sessions", "", common.RoleAdminUser)
	require.Equal(t, true, all["success"])
	require.EqualValues(t, 2, all["data"].(map[string]interface{})["total"])
	require.Equal(t, false, request("PUT", "/api/log/sessions/owner-session/group", `{}`, common.RoleAdminUser)["success"])
	require.Equal(t, true, request("PUT", "/api/log/sessions/owner-session/group", `{"group":""}`, common.RoleAdminUser)["success"])
	stored, err = model.GetActiveRelaySession("owner-session")
	require.NoError(t, err)
	require.Empty(t, stored.OverrideGroup)
	require.Equal(t, false, request("GET", "/api/log/self/sessions", "", 0)["success"])
}

func TestRelaySessionOverrideControlsDistributedChannel(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.RelaySession{}, &model.Channel{}))
	oldRatios, oldUsable := ratio_setting.GroupRatio2JSONString(), setting.UserUsableGroups2JSONString()
	oldMemory := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemory
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"A":1,"B":2}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"A":"A","B":"B"}`))
	require.NoError(t, model.DB.Create([]model.Channel{
		{Id: 1, Name: "A", Group: "A", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "test-A"},
		{Id: 2, Name: "B", Group: "B", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "test-B"},
	}).Error)
	require.NoError(t, model.DB.Create([]model.Ability{
		{ChannelId: 1, Group: "A", Model: "gpt-test", Enabled: true},
		{ChannelId: 2, Group: "B", Model: "gpt-test", Enabled: true},
	}).Error)
	engine := gin.New()
	engine.POST("/v1/responses", middleware.BodyStorageCleanup(), func(c *gin.Context) {
		c.Set("id", 1)
		c.Set("token_id", 1)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "A")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "A")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "A")
		if c.GetHeader("Test-Combination") != "" {
			common.SetContextKey(c, constant.ContextKeyTokenModelGroupCombinationEnabled, true)
			common.SetContextKey(c, constant.ContextKeyTokenModelGroupCombinationGroups, `[{"group":"A","models":["gpt-test"]}]`)
		}
		c.Next()
	}, middleware.Distribute(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"channel_id": c.GetInt("channel_id"), "group": c.GetString("group"), "token_group": c.GetString("token_group")})
	})
	request := func(key string, combination bool) map[string]interface{} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"gpt-test","prompt_cache_key":"`+key+`"}`))
		req.Header.Set("Content-Type", "application/json")
		if combination {
			req.Header.Set("Test-Combination", "true")
		}
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var response map[string]interface{}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		return response
	}
	require.EqualValues(t, 1, request("conversation-one", false)["channel_id"])
	var session model.RelaySession
	require.NoError(t, model.DB.Where("session_key = ?", "conversation-one").First(&session).Error)
	updated, err := model.UpdateRelaySessionGroup(session.ID, "B")
	require.NoError(t, err)
	require.True(t, updated)
	result := request("conversation-one", true)
	require.EqualValues(t, 2, result["channel_id"])
	require.Equal(t, "B", result["group"])
	require.Equal(t, "B", result["token_group"])
	require.EqualValues(t, 1, request("conversation-two", false)["channel_id"])
	updated, err = model.UpdateRelaySessionGroup(session.ID, "")
	require.NoError(t, err)
	require.True(t, updated)
	require.EqualValues(t, 1, request("conversation-one", false)["channel_id"])
	stored, err := model.GetActiveRelaySession(session.ID)
	require.NoError(t, err)
	require.Equal(t, "A", stored.LastGroup)
}
