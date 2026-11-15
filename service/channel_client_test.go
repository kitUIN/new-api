package service

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func clientRequestContext(userAgent string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", userAgent)
	return c
}

func TestIsClaudeCodeRequest(t *testing.T) {
	for _, tc := range []struct {
		name, userAgent, app, beta string
		want                       bool
	}{
		{name: "official CLI", userAgent: "claude-cli/2.0.30 (external, cli)", want: true},
		{name: "case insensitive", userAgent: "Claude-Code/2.1.0", want: true},
		{name: "compact product", userAgent: "claudecode/1.0", want: true},
		{name: "multiple products", userAgent: "proxy/1.0 claude-cli/2.0", want: true},
		{name: "proxy headers", userAgent: "proxy/1.0", app: "CLI", beta: "other-beta, claude-code-20250219", want: true},
		{name: "SDK", userAgent: "Anthropic/JS 0.67.0"},
		{name: "generic CLI", app: "cli"},
		{name: "beta alone", beta: "claude-code-20250219"},
		{name: "similar product", userAgent: "not-claude-cli/1.0"},
		{name: "no headers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := clientRequestContext(tc.userAgent)
			c.Request.Header.Set("X-App", tc.app)
			c.Request.Header.Set("Anthropic-Beta", tc.beta)
			require.Equal(t, tc.want, IsClaudeCodeRequest(c))
		})
	}
	require.False(t, IsClaudeCodeRequest(nil))
	require.False(t, IsClaudeCodeRequest(&gin.Context{}))
}

func TestIsCodexRequest(t *testing.T) {
	for _, tc := range []struct {
		name, userAgent, originator string
		want                        bool
	}{
		{name: "official CLI", userAgent: "codex_cli_rs/0.100.0 (Windows 11; x86_64)", want: true},
		{name: "exec", userAgent: "codex_exec/0.100.0", want: true},
		{name: "IDE", userAgent: "codex_vscode/0.100.0", want: true},
		{name: "desktop", userAgent: "Codex/1.0", want: true},
		{name: "multiple products", userAgent: "proxy/1.0 codex-cli/1.0", want: true},
		{name: "forwarded originator", userAgent: "proxy/1.0", originator: " CODEX_CLI_RS ", want: true},
		{name: "desktop originator", originator: "codex_desktop", want: true},
		{name: "app originator", originator: "codex_app", want: true},
		{name: "configured originator", originator: "Codex CLI", want: true},
		{name: "ordinary SDK", userAgent: "OpenAI/Python 1.0"},
		{name: "similar product", userAgent: "not-codex/1.0"},
		{name: "similar originator", originator: "codex_custom"},
		{name: "generic CLI", originator: "cli"},
		{name: "empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := clientRequestContext(tc.userAgent)
			c.Request.Header.Set("Originator", tc.originator)
			require.Equal(t, tc.want, IsCodexRequest(c))
		})
	}
	require.False(t, IsCodexRequest(nil))
	require.False(t, IsCodexRequest(&gin.Context{}))
}

func TestChannelClientAccessCombinations(t *testing.T) {
	// Each row specifies access for ordinary, Codex, Claude Code, and dual-identity requests.
	for _, tc := range []struct {
		codex, claude dto.ChannelClientAccess
		allowed       [4]bool
	}{
		{"unrestricted", "unrestricted", [4]bool{true, true, true, true}},
		{"deny", "unrestricted", [4]bool{true, false, true, false}},
		{"unrestricted", "deny", [4]bool{true, true, false, false}},
		{"deny", "deny", [4]bool{true, false, false, false}},
		{"allow_only", "unrestricted", [4]bool{false, true, false, true}},
		{"unrestricted", "allow_only", [4]bool{false, false, true, true}},
		{"allow_only", "allow_only", [4]bool{false, true, true, true}},
		{"allow_only", "deny", [4]bool{false, true, false, false}},
		{"deny", "allow_only", [4]bool{false, false, true, false}},
	} {
		t.Run(string(tc.codex)+"/"+string(tc.claude), func(t *testing.T) {
			channel := &model.Channel{}
			channel.SetOtherSettings(dto.ChannelOtherSettings{ClientRestrictions: &dto.ChannelClientRestrictions{Codex: tc.codex, ClaudeCode: tc.claude}})
			for i, ua := range []string{"OpenAI/Python 1.0", "codex_cli_rs/1.0", "claude-cli/2.0", "codex_cli_rs/1.0 claude-cli/2.0"} {
				require.Equal(t, !tc.allowed[i], IsChannelBlockedForClient(clientRequestContext(ua), channel), ua)
			}
		})
	}
	require.False(t, IsChannelBlockedForClient(nil, nil))
	legacy := &model.Channel{OtherSettings: `{"disable_claude_code":true}`}
	require.True(t, IsChannelBlockedForClient(clientRequestContext("claude-cli/2.0"), legacy))
	legacy.OtherSettings = `{"disable_claude_code":true,"client_restrictions":{}}`
	require.False(t, IsChannelBlockedForClient(clientRequestContext("claude-cli/2.0"), legacy))
	legacy.OtherSettings = `{"client_restrictions":{"codex":"allow_only"}}`
	require.True(t, IsChannelBlockedForClient(nil, legacy))
}

func TestClientChannelSelection(t *testing.T) {
	for _, tc := range []struct {
		name, settings, blockedUA, allowedUA string
	}{
		{"legacy Claude Code", `{"disable_claude_code":true}`, "claude-cli/2.0", "Anthropic/JS 0.67.0"},
		{"deny Codex", `{"client_restrictions":{"codex":"deny"}}`, "codex_cli_rs/1.0", "OpenAI/Python 1.0"},
		{"deny both", `{"client_restrictions":{"codex":"deny","claude_code":"deny"}}`, "claude-cli/2.0", "OpenAI/Python 1.0"},
		{"only Codex", `{"client_restrictions":{"codex":"allow_only"}}`, "OpenAI/Python 1.0", "codex_cli_rs/1.0"},
		{"only Claude Code", `{"client_restrictions":{"claude_code":"allow_only"}}`, "codex_cli_rs/1.0", "claude-cli/2.0"},
		{"only both", `{"client_restrictions":{"codex":"allow_only","claude_code":"allow_only"}}`, "", "claude-cli/2.0"},
		{"mixed rules", `{"client_restrictions":{"codex":"allow_only","claude_code":"deny"}}`, "claude-cli/2.0 codex_cli_rs/1.0", "codex_cli_rs/1.0"},
	} {
		t.Run(tc.name, func(t *testing.T) { testClientChannelSelection(t, tc.settings, tc.blockedUA, tc.allowedUA) })
	}
}

func testClientChannelSelection(t *testing.T, settingsJSON, blockedUA, allowedUA string) {
	originalDB := model.DB
	originalCache := common.MemoryCacheEnabled
	originalSQLite, originalMySQL, originalPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	originalCombinations := ratio_setting.GroupCombinations2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalCache
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = originalSQLite, originalMySQL, originalPostgres
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(originalCombinations))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		if originalDB != nil && originalCache {
			model.InitChannelCache()
		}
	})
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	blocked := &model.Channel{Id: 1, Name: "blocked", Key: "test", Status: common.ChannelStatusEnabled, Group: "client-main,client-blocked", Models: "claude-test", Priority: common.GetPointer(int64(10))}
	blocked.OtherSettings = settingsJSON
	initialSettings := blocked.GetOtherSettings()
	initialSettings.AllowSpeed = true
	blocked.SetOtherSettings(initialSettings)
	allowed := &model.Channel{Id: 2, Name: "allowed", Key: "test", Status: common.ChannelStatusEnabled, Group: "client-main", Models: "claude-test", Priority: common.GetPointer(int64(5))}
	for _, channel := range []*model.Channel{blocked, allowed} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(`{"client-combo":[{"group":"client-blocked","models":["claude-test"]},{"group":"client-main","models":["claude-test"]}]}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["client-blocked","client-main"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"client-blocked":"blocked","client-main":"main"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"client-blocked":1,"client-main":1}`))

	for _, cacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache=%t", cacheEnabled), func(t *testing.T) {
			common.MemoryCacheEnabled = cacheEnabled
			model.InitChannelCache()
			for _, group := range []string{"client-main", "client-combo", "auto"} {
				t.Run(group, func(t *testing.T) {
					param := &RetryParam{Ctx: clientRequestContext(blockedUA), TokenGroup: group, ModelName: "claude-test"}
					channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
					require.NoError(t, err)
					require.NotNil(t, channel)
					require.Equal(t, 2, channel.Id)
					require.Equal(t, "client-main", selectedGroup)
				})
			}

			t.Run("retry cannot release client restrictions", func(t *testing.T) {
				param := &RetryParam{Ctx: clientRequestContext(blockedUA), TokenGroup: "client-main", ModelName: "claude-test", ExcludedChannelIDs: []int{1, 2}}
				for retry := 0; retry < 3; retry++ {
					param.SetRetry(retry)
					channel, _, err := CacheGetRandomSatisfiedChannel(param)
					require.NoError(t, err)
					require.NotNil(t, channel)
					require.Equal(t, 2, channel.Id)
					param.ExcludeChannel(channel.Id)
				}
			})

			t.Run("all blocked", func(t *testing.T) {
				param := &RetryParam{Ctx: clientRequestContext(blockedUA), TokenGroup: "client-blocked", ModelName: "claude-test", ExcludedChannelIDs: []int{1}}
				channel, _, err := CacheGetRandomSatisfiedChannel(param)
				require.NoError(t, err)
				require.Nil(t, channel)
			})

			t.Run("allowed client keeps original priority", func(t *testing.T) {
				param := &RetryParam{Ctx: clientRequestContext(allowedUA), TokenGroup: "client-main", ModelName: "claude-test"}
				channel, _, err := CacheGetRandomSatisfiedChannel(param)
				require.NoError(t, err)
				require.NotNil(t, channel)
				require.Equal(t, 1, channel.Id)
			})

			t.Run("token model combination", func(t *testing.T) {
				c := clientRequestContext(blockedUA)
				common.SetContextKey(c, constant.ContextKeyTokenModelGroupCombinationEnabled, true)
				common.SetContextKey(c, constant.ContextKeyTokenModelGroupCombinationGroups, `[{"group":"client-blocked","models":["claude-test"]},{"group":"client-main","models":["claude-test"]}]`)
				enabled, err := PrepareModelGroupCombination(c, "claude-test")
				require.NoError(t, err)
				require.True(t, enabled)
				channel, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: c, TokenGroup: "client-blocked", ModelName: "claude-test"})
				require.NoError(t, err)
				require.NotNil(t, channel)
				require.Equal(t, 2, channel.Id)
				require.Equal(t, "client-main", group)
			})
		})
	}

	// Persisting false must restore access and preserve unrelated settings.
	settings := blocked.GetOtherSettings()
	settings.DisableClaudeCode = false
	settings.ClientRestrictions = nil
	blocked.SetOtherSettings(settings)
	require.NoError(t, db.Model(blocked).Update("settings", blocked.OtherSettings).Error)
	model.InitChannelCache()
	stored, err := model.CacheGetChannel(blocked.Id)
	require.NoError(t, err)
	require.False(t, IsChannelBlockedForClient(clientRequestContext(blockedUA), stored))
	require.True(t, stored.GetOtherSettings().AllowSpeed)
}
