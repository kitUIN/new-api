package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRelaySessionTest(t *testing.T) {
	t.Helper()
	setupModelGroupCombinationSettings(t)
	previousDB, previousMemory := model.DB, common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB, common.MemoryCacheEnabled = db, false
	t.Cleanup(func() { model.DB, common.MemoryCacheEnabled = previousDB, previousMemory; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.RelaySession{}, &model.User{}, &model.Ability{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "owner", Group: "group-a"}).Error)
	require.NoError(t, db.Create([]model.Ability{
		{Group: "group-a", Model: "gpt-test", ChannelId: 1, Enabled: true},
		{Group: "group-b", Model: "gpt-test", ChannelId: 2, Enabled: true},
	}).Error)
}

func relaySessionTestContext(userID, tokenID int, key string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", strings.NewReader(fmt.Sprintf(`{"model":"gpt-test","prompt_cache_key":%q}`, key)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userID)
	c.Set("token_id", tokenID)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "group-a")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "group-a")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "group-a")
	return c
}

func TestRelaySessionSwitchIsConversationUserAndTokenScoped(t *testing.T) {
	setupRelaySessionTest(t)
	c := relaySessionTestContext(1, 11, "shared-key")
	manual, err := PrepareRelaySession(c, "gpt-test")
	require.NoError(t, err)
	require.False(t, manual)
	value, _ := c.Get(relaySessionContextKey)
	session := value.(*model.RelaySession)
	require.NoError(t, SetRelaySessionGroup(session, "group-b"))

	for _, tc := range []struct {
		userID, tokenID int
		key             string
		manual          bool
	}{
		{1, 11, "shared-key", true},
		{2, 11, "shared-key", false},
		{1, 12, "shared-key", false},
		{1, 11, "other-key", false},
	} {
		c := relaySessionTestContext(tc.userID, tc.tokenID, tc.key)
		manual, err := PrepareRelaySession(c, "gpt-test")
		require.NoError(t, err)
		require.Equal(t, tc.manual, manual)
		expected := "group-a"
		if manual {
			expected = "group-b"
		}
		require.Equal(t, expected, common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		require.Equal(t, expected, common.GetContextKeyString(c, constant.ContextKeyTokenGroup))
	}

	require.NoError(t, SetRelaySessionGroup(session, ""))
	manual, err = PrepareRelaySession(relaySessionTestContext(1, 11, "shared-key"), "gpt-test")
	require.NoError(t, err)
	require.False(t, manual)
}

func TestRelaySessionRejectsInvalidOrUnavailableGroup(t *testing.T) {
	setupRelaySessionTest(t)
	c := relaySessionTestContext(1, 11, "group-validation")
	_, err := PrepareRelaySession(c, "gpt-test")
	require.NoError(t, err)
	value, _ := c.Get(relaySessionContextKey)
	session := value.(*model.RelaySession)
	for _, group := range []string{"auto", "missing", "deprecated", "group-c"} {
		require.Error(t, SetRelaySessionGroup(session, group), group)
	}
	require.NoError(t, SetRelaySessionGroup(session, "group-b"))
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", 2).Update("enabled", false).Error)
	_, err = PrepareRelaySession(relaySessionTestContext(1, 11, "group-validation"), "gpt-test")
	require.Error(t, err)
}

func TestRelaySessionIdentityDoesNotDependOnGroupOrAffinityEnabled(t *testing.T) {
	setting := operation_setting.GetChannelAffinitySetting()
	original := setting.Enabled
	t.Cleanup(func() { setting.Enabled = original })
	setting.Enabled = false
	c := relaySessionTestContext(1, 11, "stable-key")
	sourceA, keyA := relaySessionIdentity(c, "gpt-test", "group-a")
	sourceB, keyB := relaySessionIdentity(c, "gpt-test", "group-b")
	require.Equal(t, "body:prompt_cache_key", sourceA)
	require.Equal(t, "stable-key", keyA)
	require.Equal(t, sourceA, sourceB)
	require.Equal(t, keyA, keyB)
	c = relaySessionTestContext(1, 11, "")
	c.Request.Header.Set("Session_id", "header-key")
	_, key := relaySessionIdentity(c, "gpt-test", "group-a")
	require.Equal(t, "header-key", key)
	c.Request.Header.Del("Session_id")
	_, key = relaySessionIdentity(c, "gpt-test", "group-a")
	require.Empty(t, key)
}
