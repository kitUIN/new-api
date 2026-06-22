package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type userAccessTokenAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func setupUserAccessTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("failed to migrate user table: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedUserAccessTokenUser(t *testing.T, db *gorm.DB, userID int, accessToken *string) *model.User {
	t.Helper()

	user := &model.User{
		Id:          userID,
		Username:    fmt.Sprintf("access-token-user-%d", userID),
		Password:    "password",
		DisplayName: fmt.Sprintf("access-token-user-%d", userID),
		Status:      common.UserStatusEnabled,
		AccessToken: accessToken,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func decodeUserAccessTokenAPIResponse(t *testing.T, body []byte) userAccessTokenAPIResponse {
	t.Helper()

	var response userAccessTokenAPIResponse
	if err := common.Unmarshal(body, &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func TestGetAccessTokenGeneratesMissingToken(t *testing.T) {
	db := setupUserAccessTokenControllerTestDB(t)
	user := seedUserAccessTokenUser(t, db, 1, nil)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/token/self", nil, user.Id)
	GetAccessToken(ctx)

	response := decodeUserAccessTokenAPIResponse(t, recorder.Body.Bytes())
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if response.Data == "" {
		t.Fatalf("expected generated access token")
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.GetAccessToken() != response.Data {
		t.Fatalf("expected generated token to be persisted, got %q want %q", reloaded.GetAccessToken(), response.Data)
	}
}

func TestGetAccessTokenReturnsExistingToken(t *testing.T) {
	db := setupUserAccessTokenControllerTestDB(t)
	existingToken := "existing-access-token"
	user := seedUserAccessTokenUser(t, db, 1, &existingToken)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/token/self", nil, user.Id)
	GetAccessToken(ctx)

	response := decodeUserAccessTokenAPIResponse(t, recorder.Body.Bytes())
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	if response.Data != existingToken {
		t.Fatalf("expected existing token %q, got %q", existingToken, response.Data)
	}
}
