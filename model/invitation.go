package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	InvitationStatusAvailable = 1
	InvitationStatusUsed      = 2
)

var (
	ErrInvitationNotFound    = errors.New("invitation not found")
	ErrInvitationUsed        = errors.New("invitation already used")
	ErrInvitationUnavailable = errors.New("invitation unavailable")
)

type Invitation struct {
	Id          int    `json:"id"`
	Code        string `json:"code" gorm:"type:varchar(20);uniqueIndex"`
	Remark      string `json:"remark" gorm:"type:varchar(255)"`
	Status      int    `json:"status" gorm:"type:int;default:1;index"`
	CreatedBy   int    `json:"created_by" gorm:"type:int;index"`
	CreatedTime int64  `json:"created_time" gorm:"bigint;index"`
	UsedUserId  int    `json:"used_user_id" gorm:"type:int;index"`
	UsedTime    int64  `json:"used_time" gorm:"bigint"`
}

func GetInvitations(keyword string, startIdx int, num int) (invitations []*Invitation, total int64, err error) {
	query := DB.Model(&Invitation{})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR remark LIKE ?", like, like)
	}
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&invitations).Error
	return invitations, total, err
}

func (invitation *Invitation) Insert() error {
	return DB.Create(invitation).Error
}

func InvitationCodeExists(code string) (bool, error) {
	var count int64
	err := DB.Model(&Invitation{}).Where("code = ?", strings.TrimSpace(code)).Count(&count).Error
	return count > 0, err
}

func DeleteInvitationById(id int) error {
	var invitation Invitation
	if err := DB.First(&invitation, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvitationNotFound
		}
		return err
	}
	if invitation.Status == InvitationStatusUsed {
		return ErrInvitationUsed
	}
	return DB.Delete(&invitation).Error
}

func RegisterUserWithInvitation(user *User, inviteCode string) error {
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return ErrInvitationNotFound
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var invitation Invitation
		if err := tx.Where("code = ?", inviteCode).First(&invitation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvitationNotFound
			}
			return err
		}
		if invitation.Status == InvitationStatusUsed {
			return ErrInvitationUsed
		}
		if invitation.Status != InvitationStatusAvailable {
			return ErrInvitationUnavailable
		}

		user.Username = invitation.Code
		user.QQId = invitation.Code
		user.Remark = invitation.Remark
		var existingUsers int64
		if err := tx.Unscoped().Model(&User{}).Where("username = ?", invitation.Code).Count(&existingUsers).Error; err != nil {
			return err
		}
		if existingUsers > 0 {
			return ErrInvitationUnavailable
		}
		var existingQQUsers int64
		if err := tx.Unscoped().Model(&User{}).Where("qq_id = ?", user.QQId).Count(&existingQQUsers).Error; err != nil {
			return err
		}
		if existingQQUsers > 0 {
			return ErrUserQQAlreadyTaken
		}

		if err := user.InsertWithTx(tx, 0); err != nil {
			return err
		}

		result := tx.Model(&Invitation{}).
			Where("id = ? AND status = ?", invitation.Id, InvitationStatusAvailable).
			Updates(map[string]interface{}{
				"status":       InvitationStatusUsed,
				"used_user_id": user.Id,
				"used_time":    common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvitationUnavailable
		}
		return nil
	})
	if err != nil {
		return err
	}

	user.FinalizeCreation(0)
	return nil
}
