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

func createInvitationTestInviter(t *testing.T, db *gorm.DB) User {
	t.Helper()
	inviter := User{
		Username:    "inviter-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Password:    "hashed-password",
		DisplayName: "Inviter",
		AffCode:     "inviter-code",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	if err := db.Create(&inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}
	return inviter
}

func TestRegisterUserWithInvitationConsumesCodeAndSetsInviter(t *testing.T) {
	db := setupInvitationTestDB(t)
	inviter := createInvitationTestInviter(t, db)
	invitation := Invitation{
		Code:        "12345678",
		Remark:      "渠道合作伙伴",
		InviterId:   inviter.Id,
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
	if user.Remark != "" {
		t.Fatalf("invitation remark must not be copied to the user, got %q", user.Remark)
	}
	if user.InviterId != inviter.Id {
		t.Fatalf("expected inviter ID %d, got %d", inviter.Id, user.InviterId)
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
	inviter := createInvitationTestInviter(t, db)
	invitation := Invitation{
		Code:        "22334455",
		Remark:      "冲突测试",
		InviterId:   inviter.Id,
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
		AffCode:     "conflict-code",
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
	inviter := createInvitationTestInviter(t, db)
	invitation := Invitation{
		Code:        "11223344",
		Remark:      "QQ 冲突测试",
		InviterId:   inviter.Id,
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
		AffCode:     "qq-conflict-code",
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

func TestMigrateInvitationInvitersMovesLegacyRemark(t *testing.T) {
	db := setupInvitationTestDB(t)
	inviter := createInvitationTestInviter(t, db)
	invitedUser := User{
		Username:    "legacy-invitee",
		Password:    "hashed-password",
		DisplayName: "Legacy Invitee",
		Remark:      fmt.Sprintf("%d", inviter.Id),
		AffCode:     "legacy-code",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	if err := db.Create(&invitedUser).Error; err != nil {
		t.Fatalf("failed to create legacy invited user: %v", err)
	}

	invitation := Invitation{
		Code:        "55667788",
		Remark:      fmt.Sprintf("%d", inviter.Id),
		Status:      InvitationStatusUsed,
		CreatedTime: common.GetTimestamp(),
		UsedUserId:  invitedUser.Id,
		UsedTime:    common.GetTimestamp(),
	}
	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("failed to create legacy invitation: %v", err)
	}

	if err := MigrateInvitationInviters(); err != nil {
		t.Fatalf("failed to migrate invitation inviter: %v", err)
	}
	if err := MigrateInvitationInviters(); err != nil {
		t.Fatalf("idempotent migration failed: %v", err)
	}

	var migratedInvitation Invitation
	if err := db.First(&migratedInvitation, invitation.Id).Error; err != nil {
		t.Fatalf("failed to reload migrated invitation: %v", err)
	}
	if migratedInvitation.InviterId != inviter.Id || migratedInvitation.Remark != "" {
		t.Fatalf("unexpected migrated invitation: %+v", migratedInvitation)
	}

	var migratedUser User
	if err := db.First(&migratedUser, invitedUser.Id).Error; err != nil {
		t.Fatalf("failed to reload migrated user: %v", err)
	}
	if migratedUser.InviterId != inviter.Id || migratedUser.Remark != "" {
		t.Fatalf("unexpected migrated user: %+v", migratedUser)
	}
}

func TestMigrateInvitationInvitersPreservesUnrelatedUserRemark(t *testing.T) {
	db := setupInvitationTestDB(t)
	inviter := createInvitationTestInviter(t, db)
	invitedUser := User{
		Username:    "edited-invitee",
		Password:    "hashed-password",
		DisplayName: "Edited Invitee",
		Remark:      "manually edited",
		AffCode:     "edited-code",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	if err := db.Create(&invitedUser).Error; err != nil {
		t.Fatalf("failed to create invited user: %v", err)
	}
	invitation := Invitation{
		Code:       "66778899",
		Remark:     fmt.Sprintf("%d", inviter.Id),
		Status:     InvitationStatusUsed,
		UsedUserId: invitedUser.Id,
	}
	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("failed to create legacy invitation: %v", err)
	}

	if err := MigrateInvitationInviters(); err != nil {
		t.Fatalf("failed to migrate invitation inviter: %v", err)
	}

	var migratedUser User
	if err := db.First(&migratedUser, invitedUser.Id).Error; err != nil {
		t.Fatalf("failed to reload migrated user: %v", err)
	}
	if migratedUser.InviterId != inviter.Id || migratedUser.Remark != invitedUser.Remark {
		t.Fatalf("unrelated user remark was not preserved: %+v", migratedUser)
	}
}
