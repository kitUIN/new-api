package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupInvitationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalNewUserQuota := common.QuotaForNewUser
	originalInviteeQuota := common.QuotaForInvitee
	originalInviterQuota := common.QuotaForInviter

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 0

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db
	if err := db.AutoMigrate(&User{}, &Invitation{}); err != nil {
		t.Fatalf("failed to migrate invitation test tables: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		common.QuotaForNewUser = originalNewUserQuota
		common.QuotaForInvitee = originalInviteeQuota
		common.QuotaForInviter = originalInviterQuota
	})

	return db
}

func TestRegisterUserWithInvitationConsumesCodeAndCopiesRemark(t *testing.T) {
	db := setupInvitationTestDB(t)
	invitation := Invitation{
		Code:        "12345678",
		Remark:      "渠道合作伙伴",
		Status:      InvitationStatusAvailable,
		CreatedTime: common.GetTimestamp(),
	}
	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("failed to create invitation: %v", err)
	}

	user := User{
		Password:    "password123",
		DisplayName: "测试昵称",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	if err := RegisterUserWithInvitation(&user, invitation.Code); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if user.Id == 0 {
		t.Fatal("expected created user id")
	}
	if user.Username != invitation.Code {
		t.Fatalf("expected username %q, got %q", invitation.Code, user.Username)
	}
	if user.QQId != invitation.Code {
		t.Fatalf("expected qq id %q, got %q", invitation.Code, user.QQId)
	}
	if user.Remark != invitation.Remark {
		t.Fatalf("expected copied remark %q, got %q", invitation.Remark, user.Remark)
	}
	if user.Password == "password123" {
		t.Fatal("expected password to be hashed")
	}

	var savedInvitation Invitation
	if err := db.First(&savedInvitation, invitation.Id).Error; err != nil {
		t.Fatalf("failed to reload invitation: %v", err)
	}
	if savedInvitation.Status != InvitationStatusUsed {
		t.Fatalf("expected used status, got %d", savedInvitation.Status)
	}
	if savedInvitation.UsedUserId != user.Id || savedInvitation.UsedTime == 0 {
		t.Fatalf("unexpected invitation usage data: %+v", savedInvitation)
	}

	secondUser := User{
		Password:    "password456",
		DisplayName: "另一个用户",
		Role:        common.RoleCommonUser,
	}
	if err := RegisterUserWithInvitation(&secondUser, invitation.Code); !errors.Is(err, ErrInvitationUsed) {
		t.Fatalf("expected ErrInvitationUsed, got %v", err)
	}
}

func TestRegisterUserWithInvitationLeavesCodeAvailableOnUsernameConflict(t *testing.T) {
	db := setupInvitationTestDB(t)
	invitation := Invitation{
		Code:        "22334455",
		Remark:      "冲突测试",
		Status:      InvitationStatusAvailable,
		CreatedTime: common.GetTimestamp(),
	}
	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("failed to create invitation: %v", err)
	}
	if err := db.Create(&User{
		Username:    invitation.Code,
		Password:    "hashed-password",
		DisplayName: "Existing",
		Role:        common.RoleCommonUser,
	}).Error; err != nil {
		t.Fatalf("failed to create conflicting user: %v", err)
	}

	user := User{
		Password:    "password123",
		DisplayName: "New User",
		Role:        common.RoleCommonUser,
	}
	if err := RegisterUserWithInvitation(&user, invitation.Code); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatalf("expected ErrInvitationUnavailable, got %v", err)
	}

	var savedInvitation Invitation
	if err := db.First(&savedInvitation, invitation.Id).Error; err != nil {
		t.Fatalf("failed to reload invitation: %v", err)
	}
	if savedInvitation.Status != InvitationStatusAvailable || savedInvitation.UsedUserId != 0 {
		t.Fatalf("invitation should remain available after rollback: %+v", savedInvitation)
	}
}

func TestDeleteInvitationRejectsUsedCode(t *testing.T) {
	db := setupInvitationTestDB(t)
	invitation := Invitation{
		Code:        "33445566",
		Remark:      "已使用",
		Status:      InvitationStatusUsed,
		CreatedTime: common.GetTimestamp(),
		UsedUserId:  1,
		UsedTime:    common.GetTimestamp(),
	}
	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("failed to create invitation: %v", err)
	}

	if err := DeleteInvitationById(invitation.Id); !errors.Is(err, ErrInvitationUsed) {
		t.Fatalf("expected ErrInvitationUsed, got %v", err)
	}
}

func TestRegisterUserWithInvitationRejectsDuplicateQQWithoutConsumingCode(t *testing.T) {
	db := setupInvitationTestDB(t)
	invitation := Invitation{
		Code:        "11223344",
		Remark:      "QQ 冲突测试",
		Status:      InvitationStatusAvailable,
		CreatedTime: common.GetTimestamp(),
	}
	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("failed to create invitation: %v", err)
	}
	if err := db.Create(&User{
		Username:    "existing-qq-user",
		Password:    "hashed-password",
		DisplayName: "Existing QQ",
		QQId:        invitation.Code,
		Role:        common.RoleCommonUser,
	}).Error; err != nil {
		t.Fatalf("failed to create existing QQ user: %v", err)
	}

	user := User{
		Password:    "password123",
		DisplayName: "New User",
		Role:        common.RoleCommonUser,
	}
	if err := RegisterUserWithInvitation(&user, invitation.Code); !errors.Is(err, ErrUserQQAlreadyTaken) {
		t.Fatalf("expected ErrUserQQAlreadyTaken, got %v", err)
	}

	var savedInvitation Invitation
	if err := db.First(&savedInvitation, invitation.Id).Error; err != nil {
		t.Fatalf("failed to reload invitation: %v", err)
	}
	if savedInvitation.Status != InvitationStatusAvailable || savedInvitation.UsedUserId != 0 {
		t.Fatalf("invitation should remain available after QQ conflict: %+v", savedInvitation)
	}
}
