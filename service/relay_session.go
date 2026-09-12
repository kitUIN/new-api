package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const relaySessionContextKey = "relay_session"

var relaySessionLastCleanup atomic.Int64

func relaySessionIdentity(c *gin.Context, modelName, group string) (string, string) {
	if meta, ok := resolveChannelAffinityMeta(c, modelName, group, false); ok {
		// Use the original value, never the group-dependent affinity cache key.
		if meta.KeySourceType == "gjson" {
			return "body:" + meta.KeySourcePath, extractChannelAffinityValue(c, operation_setting.ChannelAffinityKeySource{Type: "gjson", Path: meta.KeySourcePath})
		}
	}
	for _, header := range []string{"Session_id", "X-Session-ID", "X-Conversation-ID"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return "header:" + header, value
		}
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return "", ""
	}
	body, err := storage.Bytes()
	if err != nil {
		return "", ""
	}
	for _, path := range []string{"session_id", "conversation_id", "prompt_cache_key", "metadata.session_id", "metadata.user_id"} {
		value := gjson.GetBytes(body, path)
		if value.Type == gjson.String && strings.TrimSpace(value.String()) != "" {
			return "body:" + path, strings.TrimSpace(value.String())
		}
	}
	return "", ""
}

// PrepareRelaySession runs before automatic group selection. A manual override
// updates both routing and billing context and bypasses automatic group policies.
func PrepareRelaySession(c *gin.Context, modelName string) (bool, error) {
	userID := c.GetInt("id")
	tokenID := c.GetInt("token_id")
	if userID <= 0 || tokenID <= 0 || modelName == "" {
		return false, nil
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	source, value := relaySessionIdentity(c, modelName, group)
	if value == "" || len(value) > 1024 {
		return false, nil
	}
	now := time.Now().Unix()
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s:%s", userID, tokenID, source, value))))
	session := &model.RelaySession{
		ID: id, UserID: userID, Username: c.GetString("username"),
		TokenID: tokenID, TokenName: c.GetString("token_name"),
		SessionKey: value, Source: source, ModelName: modelName,
		RequestedGroup: group, LastGroup: group, CreatedAt: now, LastSeenAt: now,
	}
	if err := model.TouchRelaySession(session); err != nil {
		return false, err
	}
	c.Set(relaySessionContextKey, session)
	lastCleanup := relaySessionLastCleanup.Load()
	if now-lastCleanup >= 600 && relaySessionLastCleanup.CompareAndSwap(lastCleanup, now) {
		if err := model.DeleteExpiredRelaySessions(); err != nil {
			common.SysError("relay session cleanup failed: " + err.Error())
		}
	}
	stored, err := model.GetActiveRelaySession(id)
	if err != nil {
		return false, err
	}
	if stored.OverrideGroup == "" {
		return false, nil
	}
	if err := validateRelaySessionGroup(stored.OverrideGroup, common.GetContextKeyString(c, constant.ContextKeyUserGroup), modelName); err != nil {
		return false, err
	}
	common.SetContextKey(c, constant.ContextKeyUsingGroup, stored.OverrideGroup)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, stored.OverrideGroup)
	return true, nil
}

func RecordRelaySessionRoute(c *gin.Context) {
	value, ok := c.Get(relaySessionContextKey)
	if !ok {
		return
	}
	session, ok := value.(*model.RelaySession)
	if !ok {
		return
	}
	group := common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	if session.LastGroup == group {
		return
	}
	if err := model.RecordRelaySessionGroup(session.ID, group, session.LastSeenAt); err != nil {
		common.SysError("relay session route update failed: " + err.Error())
		return
	}
	session.LastGroup = group
}

func validateRelaySessionGroup(group, userGroup, modelName string) error {
	if !ratio_setting.ContainsGroupRatio(group) || ratio_setting.IsGroupCombination(group) || group == "auto" || IsRuleAutoGroup(group) {
		return errors.New("请选择有效的普通分组")
	}
	if !GroupInUserUsableGroups(userGroup, group) {
		return fmt.Errorf("用户无权访问 %s 分组", group)
	}
	if !model.HasAvailableChannelForGroupModel(group, modelName) {
		return fmt.Errorf("分组 %s 没有模型 %s 的可用渠道", group, modelName)
	}
	return nil
}

func GetRelaySessionGroups(session *model.RelaySession) ([]string, error) {
	user, err := model.GetUserById(session.UserID, false)
	if err != nil {
		return nil, err
	}
	groups := make([]string, 0)
	for group := range GetUserUsableGroups(user.Group) {
		if validateRelaySessionGroup(group, user.Group, session.ModelName) == nil {
			groups = append(groups, group)
		}
	}
	sort.Strings(groups)
	return groups, nil
}

func SetRelaySessionGroup(session *model.RelaySession, group string) error {
	if group != "" {
		user, err := model.GetUserById(session.UserID, false)
		if err != nil {
			return err
		}
		if err := validateRelaySessionGroup(group, user.Group, session.ModelName); err != nil {
			return err
		}
	}
	updated, err := model.UpdateRelaySessionGroup(session.ID, group)
	if err != nil {
		return err
	}
	if !updated {
		return errors.New("会话已过期，请刷新后重试")
	}
	return nil
}
