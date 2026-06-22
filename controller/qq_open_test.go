package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type qqOpenAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		QQId         string                    `json:"qq_id"`
		UserId       int                       `json:"user_id"`
		UserGroup    string                    `json:"user_group"`
		Tokens       []qqOpenTokenResponse     `json:"tokens"`
		Token        qqOpenTokenResponse       `json:"token"`
		UsableGroups map[string]map[string]any `json:"usable_groups"`
		Groups       []model.PerfGroupHealth   `json:"groups"`
	} `json:"data"`
}

func setupQQOpenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	originalQQCallbackAccessToken := common.QQCallbackAccessToken
	common.QQCallbackAccessToken = "qq-open-test-token"
	t.Cleanup(func() {
		common.QQCallbackAccessToken = originalQQCallbackAccessToken
	})

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.PerfMetricBucket{}); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedQQOpenUser(t *testing.T, db *gorm.DB, userId int, qqId string, group string) *model.User {
	t.Helper()

	user := &model.User{
		Id:          userId,
		Username:    fmt.Sprintf("qq-open-user-%d", userId),
		Password:    "password",
		DisplayName: fmt.Sprintf("qq-open-user-%d", userId),
		Status:      common.UserStatusEnabled,
		Group:       group,
		QQId:        qqId,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func newQQOpenContext(t *testing.T, method string, target string, body any, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	requestBody := bytes.NewReader(nil)
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	ctx.Request.Header.Set("X-Access-Token", common.QQCallbackAccessToken)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Params = params
	return ctx, recorder
}

func decodeQQOpenAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) qqOpenAPIResponse {
	t.Helper()

	var response qqOpenAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func TestGetQQUserTokensMasksKeysAndUsesBoundQQ(t *testing.T) {
	db := setupQQOpenControllerTestDB(t)
	user := seedQQOpenUser(t, db, 1, "10001", "default")
	token := seedToken(t, db, user.Id, "qq-token", "qqraw1234token5678")
	seedToken(t, db, 2, "other-token", "other1234token5678")

	ctx, recorder := newQQOpenContext(t, http.MethodGet, "/api/qq/users/10001/tokens", nil, gin.Params{{Key: "qq_id", Value: user.QQId}})
	GetQQUserTokens(ctx)

	response := decodeQQOpenAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if response.Data.QQId != user.QQId || response.Data.UserId != user.Id {
		t.Fatalf("unexpected user mapping: %+v", response.Data)
	}
	if len(response.Data.Tokens) != 1 {
		t.Fatalf("expected one token, got %d", len(response.Data.Tokens))
	}
	if response.Data.Tokens[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked key %q, got %q", token.GetMaskedKey(), response.Data.Tokens[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("qq token list leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetQQUserGroupsReturnsCurrentUsableGroups(t *testing.T) {
	db := setupQQOpenControllerTestDB(t)
	user := seedQQOpenUser(t, db, 1, "10002", "default")
	if err := db.Create(&model.Channel{
		Type:   1,
		Name:   "default-channel",
		Key:    "channel-key",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "gpt-test",
	}).Error; err != nil {
		t.Fatalf("failed to seed channel: %v", err)
	}

	ctx, recorder := newQQOpenContext(t, http.MethodGet, "/api/qq/users/10002/groups", nil, gin.Params{{Key: "qq_id", Value: user.QQId}})
	GetQQUserGroups(ctx)

	response := decodeQQOpenAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if response.Data.UserGroup != "default" {
		t.Fatalf("expected user group default, got %q", response.Data.UserGroup)
	}
	if _, ok := response.Data.UsableGroups["default"]; !ok {
		t.Fatalf("expected default group to be usable, got %+v", response.Data.UsableGroups)
	}
}

func TestGetQQUserGroupsSkipsGroupsWithoutEnabledChannels(t *testing.T) {
	db := setupQQOpenControllerTestDB(t)
	user := seedQQOpenUser(t, db, 1, "100021", "default")
	if err := db.Create(&model.Channel{
		Type:   1,
		Name:   "disabled-channel",
		Key:    "channel-key",
		Status: common.ChannelStatusManuallyDisabled,
		Group:  "default",
		Models: "gpt-test",
	}).Error; err != nil {
		t.Fatalf("failed to seed channel: %v", err)
	}

	ctx, recorder := newQQOpenContext(t, http.MethodGet, "/api/qq/users/100021/groups", nil, gin.Params{{Key: "qq_id", Value: user.QQId}})
	GetQQUserGroups(ctx)

	response := decodeQQOpenAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if _, ok := response.Data.UsableGroups["default"]; ok {
		t.Fatalf("expected default group to be skipped without enabled channels, got %+v", response.Data.UsableGroups)
	}
}

func TestUpdateQQUserTokenGroupRequiresOwnership(t *testing.T) {
	db := setupQQOpenControllerTestDB(t)
	user := seedQQOpenUser(t, db, 1, "10003", "default")
	otherToken := seedToken(t, db, 2, "other-user-token", "otherqq1234token5678")

	ctx, recorder := newQQOpenContext(
		t,
		http.MethodPut,
		"/api/qq/users/10003/tokens/"+strconv.Itoa(otherToken.Id)+"/group",
		map[string]any{"group": "default"},
		gin.Params{
			{Key: "qq_id", Value: user.QQId},
			{Key: "token_id", Value: strconv.Itoa(otherToken.Id)},
		},
	)
	UpdateQQUserTokenGroup(ctx)

	response := decodeQQOpenAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected cross-user token update to fail")
	}
	var reloaded model.Token
	if err := db.First(&reloaded, otherToken.Id).Error; err != nil {
		t.Fatalf("failed to reload token: %v", err)
	}
	if reloaded.Group != "default" {
		t.Fatalf("expected other token group to remain unchanged, got %q", reloaded.Group)
	}
}

func TestUpdateQQUserTokenGroupUpdatesOwnedToken(t *testing.T) {
	db := setupQQOpenControllerTestDB(t)
	user := seedQQOpenUser(t, db, 1, "10004", "default")
	token := seedToken(t, db, user.Id, "owned-token", "ownedqq1234token5678")
	token.Group = "vip"
	if err := db.Save(token).Error; err != nil {
		t.Fatalf("failed to update seed token: %v", err)
	}

	ctx, recorder := newQQOpenContext(
		t,
		http.MethodPut,
		"/api/qq/users/10004/tokens/"+strconv.Itoa(token.Id)+"/group",
		map[string]any{"group": "default"},
		gin.Params{
			{Key: "qq_id", Value: user.QQId},
			{Key: "token_id", Value: strconv.Itoa(token.Id)},
		},
	)
	UpdateQQUserTokenGroup(ctx)

	response := decodeQQOpenAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected owned token update to succeed, got message: %s", response.Message)
	}
	if response.Data.Token.Group != "default" {
		t.Fatalf("expected response token group default, got %q", response.Data.Token.Group)
	}
	var reloaded model.Token
	if err := db.First(&reloaded, token.Id).Error; err != nil {
		t.Fatalf("failed to reload token: %v", err)
	}
	if reloaded.Group != "default" {
		t.Fatalf("expected token group default, got %q", reloaded.Group)
	}
}

func TestGetQQGroupHealthSummaryUsesQQOpenAuth(t *testing.T) {
	db := setupQQOpenControllerTestDB(t)
	channel := &model.Channel{
		Type:    1,
		Name:    "default-channel",
		Key:     "channel-key",
		Status:  common.ChannelStatusEnabled,
		Group:   "default",
		Balance: 10,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	now := time.Now().Unix()
	bucketStart := now - now%600
	bucket := &model.PerfMetricBucket{
		BucketStart:      bucketStart,
		BucketSeconds:    600,
		ModelName:        "gpt-test",
		Group:            "default",
		RequestCount:     10,
		SuccessCount:     9,
		LatencyCount:     9,
		TotalLatencyMs:   900,
		TotalTTFTMs:      450,
		TTFTCount:        9,
		CompletionTokens: 90,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(bucket).Error; err != nil {
		t.Fatalf("failed to create perf bucket: %v", err)
	}

	ctx, recorder := newQQOpenContext(t, http.MethodGet, "/api/qq/group-health?hours=1&interval_minutes=10", nil, nil)
	GetQQGroupHealthSummary(ctx)

	response := decodeQQOpenAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	var defaultGroup *model.PerfGroupHealth
	for i := range response.Data.Groups {
		if response.Data.Groups[i].Group == "default" {
			defaultGroup = &response.Data.Groups[i]
			break
		}
	}
	if defaultGroup == nil {
		t.Fatalf("expected default group health in response: %+v", response.Data.Groups)
	}
	if defaultGroup.SuccessRate != 90 {
		t.Fatalf("expected default group success rate 90, got %v", defaultGroup.SuccessRate)
	}
}
