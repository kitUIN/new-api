package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDistributeClaudeCodeRestriction(t *testing.T) {
	require.NoError(t, i18n.Init())
	originalDB, originalCache := model.DB, common.MemoryCacheEnabled
	originalSQLite, originalMySQL, originalPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	affinitySetting := operation_setting.GetChannelAffinitySetting()
	originalAffinity := *affinitySetting
	*affinitySetting = operation_setting.ChannelAffinitySetting{
		Enabled: true, DefaultTTLSeconds: 60,
		Rules: []operation_setting.ChannelAffinityRule{{
			Name: "client restriction test", ModelRegex: []string{"^claude-test$"},
			KeySources:        []operation_setting.ChannelAffinityKeySource{{Type: "context_string", Key: "client_test_session"}},
			IncludeUsingGroup: true,
		}},
	}
	t.Cleanup(func() {
		*affinitySetting = originalAffinity
		model.DB, common.MemoryCacheEnabled = originalDB, originalCache
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = originalSQLite, originalMySQL, originalPostgres
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
	})
	common.MemoryCacheEnabled = false
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	channel := &model.Channel{Id: 1, Type: constant.ChannelTypeAnthropic, Key: "test-key", Status: common.ChannelStatusEnabled, Group: "client-test,client-blocked", Models: "claude-test", Priority: common.GetPointer(int64(10)), OtherSettings: `{"disable_claude_code":true}`}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	allowed := &model.Channel{Id: 2, Type: constant.ChannelTypeAnthropic, Key: "test-key", Status: common.ChannelStatusEnabled, Group: "client-test", Models: "claude-test", Priority: common.GetPointer(int64(5))}
	require.NoError(t, db.Create(allowed).Error)
	require.NoError(t, allowed.AddAbilities(nil))
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(`{"client-fixed":{"claude-test":1}}`))

	for _, tc := range []struct {
		name, userAgent, group, specific string
		affinity                         bool
		status                           int
	}{
		{name: "explicit channel", userAgent: "claude-cli/2.0", group: "client-test", specific: "1", status: http.StatusForbidden},
		{name: "legacy fixed route", userAgent: "claude-cli/2.0", group: "client-fixed", status: http.StatusForbidden},
		{name: "automatic all blocked", userAgent: "claude-cli/2.0", group: "client-blocked", status: http.StatusServiceUnavailable},
		{name: "automatic skips blocked", userAgent: "claude-cli/2.0", group: "client-test", status: http.StatusOK},
		{name: "affinity skips blocked", userAgent: "claude-cli/2.0", group: "client-test", status: http.StatusOK, affinity: true},
		{name: "ordinary SDK allowed", userAgent: "Anthropic/JS 0.67.0", group: "client-test", specific: "1", status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("User-Agent", tc.userAgent)
			common.SetContextKey(c, constant.ContextKeyUsingGroup, tc.group)
			if tc.affinity {
				c.Set("client_test_session", t.Name())
				service.GetPreferredChannelByAffinity(c, "claude-test", tc.group)
				service.RecordChannelAffinity(c, channel.Id)
				preferredID, found := service.GetPreferredChannelByAffinity(c, "claude-test", tc.group)
				require.True(t, found)
				require.Equal(t, channel.Id, preferredID)
			}
			if tc.specific != "" {
				common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, tc.specific)
			}
			Distribute()(c)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == http.StatusOK {
				require.False(t, c.IsAborted())
				require.Equal(t, "test-key", c.GetString("channel_key"))
				if tc.specific == "" {
					require.Equal(t, allowed.Id, c.GetInt("channel_id"))
				}
			} else {
				require.True(t, c.IsAborted())
				require.Empty(t, c.GetString("channel_key"))
			}
			if tc.status == http.StatusForbidden {
				require.Contains(t, w.Body.String(), string(types.ErrorCodeClaudeCodeNotAllowed))
			}
		})
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.0")
	setupErr := SetupContextForSelectedChannel(c, channel, "claude-test")
	require.NotNil(t, setupErr)
	require.Equal(t, http.StatusForbidden, setupErr.StatusCode)
	require.True(t, types.IsSkipRetryError(setupErr))
}
