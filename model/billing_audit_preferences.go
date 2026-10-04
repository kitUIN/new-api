package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BillingAuditPreference stores the shared accounting exclusions for one month.
type BillingAuditPreference struct {
	Month          string `gorm:"primaryKey;size:7"`
	ExcludedGroups string `gorm:"type:text;not null"`
	UpdatedBy      int
	UpdatedAt      int64
}

func GetBillingAuditExcludedGroups(month string) ([]string, error) {
	var preference BillingAuditPreference
	err := DB.Where("month = ?", month).First(&preference).Error
	groups := make([]string, 0)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return groups, nil
	}
	if err != nil {
		return nil, err
	}
	if err := common.UnmarshalJsonStr(preference.ExcludedGroups, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func SaveBillingAuditExcludedGroups(month string, groups []string, actor int) error {
	encoded, err := common.Marshal(groups)
	if err != nil {
		return err
	}
	preference := BillingAuditPreference{Month: month, ExcludedGroups: string(encoded), UpdatedBy: actor, UpdatedAt: time.Now().Unix()}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "month"}},
		DoUpdates: clause.AssignmentColumns([]string{"excluded_groups", "updated_by", "updated_at"}),
	}).Create(&preference).Error
}
