package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type invitationRegisterResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupInvitationRegisterControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalRedisEnabled := common.RedisEnabled
	originalNewUserQuota := common.QuotaForNewUser
	originalInviteeQuota := common.QuotaForInvitee
	originalInviterQuota := common.QuotaForInviter
	originalGenerateDefaultToken := constant.GenerateDefaultToken

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0
	constant.GenerateDefaultToken = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	if err := db.AutoMigrate(&model.User{}, &model.Invitation{}); err != nil {
		t.Fatalf("failed to migrate invitation registration tables: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.RedisEnabled = originalRedisEnabled
		common.QuotaForNewUser = originalNewUserQuota
		common.QuotaForInvitee = originalInviteeQuota
		common.QuotaForInviter = originalInviterQuota
		constant.GenerateDefaultToken = originalGenerateDefaultToken
	})

	return db
}

func TestRegisterUsesInvitationAsUsernameAndCopiesProfileFields(t *testing.T) {
	db := setupInvitationRegisterControllerTestDB(t)
	invitation := model.Invitation{
		Code:        "123456789",
		Remark:      "企业客户 A",
		Status:      model.InvitationStatusAvailable,
		CreatedTime: common.GetTimestamp(),
	}
	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("failed to seed invitation: %v", err)
	}

	payload, err := common.Marshal(map[string]any{
		"invite_code":  "123456789",
		"display_name": "用户昵称",
		"password":     "password123",
	})
	if err != nil {
		t.Fatalf("failed to marshal registration payload: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	Register(ctx)

	var response invitationRegisterResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected successful registration, got %q", response.Message)
	}

	var user model.User
	if err := db.Where("username = ?", invitation.Code).First(&user).Error; err != nil {
		t.Fatalf("failed to load registered user: %v", err)
	}
	if user.DisplayName != "用户昵称" || user.QQId != invitation.Code || user.Remark != invitation.Remark {
		t.Fatalf("unexpected registered user fields: %+v", user)
	}

	var consumed model.Invitation
	if err := db.First(&consumed, invitation.Id).Error; err != nil {
		t.Fatalf("failed to load consumed invitation: %v", err)
	}
	if consumed.Status != model.InvitationStatusUsed || consumed.UsedUserId != user.Id {
		t.Fatalf("invitation was not consumed correctly: %+v", consumed)
	}
}

func TestRegisterAllowsInvitationWhenGeneralRegistrationDisabled(t *testing.T) {
	db := setupInvitationRegisterControllerTestDB(t)
	common.RegisterEnabled = false

	invitation := model.Invitation{
		Code:        "987654321",
		Remark:      "invitation-only user",
		Status:      model.InvitationStatusAvailable,
		CreatedTime: common.GetTimestamp(),
	}
	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("failed to seed invitation: %v", err)
	}

	payload, err := common.Marshal(map[string]any{
		"invite_code":  invitation.Code,
		"display_name": "invited user",
		"password":     "password123",
	})
	if err != nil {
		t.Fatalf("failed to marshal registration payload: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	Register(ctx)

	var response invitationRegisterResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected invitation registration to succeed when general registration is disabled, got %q", response.Message)
	}

	var user model.User
	if err := db.Where("username = ?", invitation.Code).First(&user).Error; err != nil {
		t.Fatalf("failed to load registered user: %v", err)
	}
}
