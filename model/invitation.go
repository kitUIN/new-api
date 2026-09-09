package model

import (
	"errors"
	"fmt"
	"strconv"
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
	ErrInvitationInviter     = errors.New("invitation inviter is invalid")
)

type Invitation struct {
	Id          int    `json:"id"`
	Code        string `json:"code" gorm:"type:varchar(20);uniqueIndex"`
	Remark      string `json:"remark" gorm:"type:varchar(255)"`
	InviterId   int    `json:"inviter_id" gorm:"type:int;not null;default:0;index"`
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
		if inviterId, parseErr := strconv.Atoi(keyword); parseErr == nil {
			query = query.Where("code LIKE ? OR remark LIKE ? OR inviter_id = ?", like, like, inviterId)
		} else {
			query = query.Where("code LIKE ? OR remark LIKE ?", like, like)
		}
	}
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&invitations).Error
	return invitations, total, err
}

func (invitation *Invitation) Insert() error {
	if invitation.InviterId <= 0 {
		return ErrInvitationInviter
	}
	var count int64
	if err := DB.Model(&User{}).Where("id = ?", invitation.InviterId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrInvitationInviter
	}
	return DB.Create(invitation).Error
}

// MigrateInvitationInviters moves the legacy inviter ID stored in remark into
// the dedicated inviter_id fields. It is safe to run more than once.
func MigrateInvitationInviters() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var invitations []Invitation
		if err := tx.Where("inviter_id = ?", 0).Find(&invitations).Error; err != nil {
			return err
		}

		migrated := 0
		for _, invitation := range invitations {
			legacyRemark := strings.TrimSpace(invitation.Remark)
			inviterId, err := strconv.Atoi(legacyRemark)
			if err != nil || inviterId <= 0 {
				continue
			}

			result := tx.Model(&Invitation{}).
				Where("id = ? AND inviter_id = ?", invitation.Id, 0).
				Updates(map[string]interface{}{
					"inviter_id": inviterId,
					"remark":     "",
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}

			if invitation.UsedUserId > 0 {
				if err := tx.Unscoped().Model(&User{}).
					Where("id = ? AND inviter_id = ?", invitation.UsedUserId, 0).
					Update("inviter_id", inviterId).Error; err != nil {
					return err
				}
				if err := tx.Unscoped().Model(&User{}).
					Where("id = ? AND remark = ?", invitation.UsedUserId, invitation.Remark).
					Update("remark", "").Error; err != nil {
					return err
				}
			}
			migrated++
		}

		if migrated > 0 {
			common.SysLog(fmt.Sprintf("migrated inviter IDs for %d registration invitations", migrated))
		}
		return nil
	})
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
		if invitation.InviterId <= 0 {
			return ErrInvitationUnavailable
		}

		user.Username = invitation.Code
		user.QQId = invitation.Code
		user.InviterId = invitation.InviterId
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

		if err := user.InsertWithTx(tx, invitation.InviterId); err != nil {
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

	user.FinalizeCreation(user.InviterId)
	return nil
}
