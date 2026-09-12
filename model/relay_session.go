package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const RelaySessionIdleSeconds int64 = 3600

// RelaySession identifies a client conversation within one user's API key.
// Routing overrides are separate from activity fields so requests cannot undo an admin edit.
type RelaySession struct {
	ID             string `json:"id" gorm:"primaryKey;size:64"`
	UserID         int    `json:"user_id" gorm:"index:idx_relay_session_user_seen,priority:1"`
	Username       string `json:"username" gorm:"size:64"`
	TokenID        int    `json:"token_id"`
	TokenName      string `json:"token_name" gorm:"size:255"`
	SessionKey     string `json:"session_key" gorm:"type:text"`
	Source         string `json:"source" gorm:"size:128"`
	ModelName      string `json:"model_name" gorm:"size:255"`
	RequestedGroup string `json:"requested_group" gorm:"size:255"`
	LastGroup      string `json:"last_group" gorm:"size:255"`
	OverrideGroup  string `json:"override_group" gorm:"size:255;default:''"`
	CreatedAt      int64  `json:"created_at"`
	LastSeenAt     int64  `json:"last_seen_at" gorm:"index;index:idx_relay_session_user_seen,priority:2"`
}

func TouchRelaySession(session *RelaySession) error {
	lastSeen := clause.Column{Table: "relay_sessions", Name: "last_seen_at"}
	// Reset expired overrides atomically. These assignments precede last_seen_at
	// because MySQL evaluates ON DUPLICATE KEY UPDATE assignments in order.
	updates := clause.Set{
		{Column: clause.Column{Name: "override_group"}, Value: gorm.Expr("CASE WHEN ? <= ? THEN ? ELSE ? END", lastSeen, session.LastSeenAt-RelaySessionIdleSeconds, "", clause.Column{Table: "relay_sessions", Name: "override_group"})},
		{Column: clause.Column{Name: "created_at"}, Value: gorm.Expr("CASE WHEN ? <= ? THEN ? ELSE ? END", lastSeen, session.LastSeenAt-RelaySessionIdleSeconds, session.CreatedAt, clause.Column{Table: "relay_sessions", Name: "created_at"})},
	}
	updates = append(updates, clause.AssignmentColumns([]string{"username", "token_name", "model_name", "requested_group"})...)
	updates = append(updates, clause.Assignment{Column: clause.Column{Name: "last_seen_at"}, Value: gorm.Expr("CASE WHEN ? > ? THEN ? ELSE ? END", lastSeen, session.LastSeenAt, lastSeen, session.LastSeenAt)})
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: updates,
	}).Create(session).Error
}

func GetActiveRelaySession(id string) (*RelaySession, error) {
	var session RelaySession
	err := DB.Where("id = ? AND last_seen_at > ?", id, time.Now().Unix()-RelaySessionIdleSeconds).
		First(&session).Error
	return &session, err
}

func GetActiveRelaySessions(userID int, search string, offset, limit int) ([]RelaySession, int64, error) {
	query := DB.Model(&RelaySession{}).Where("last_seen_at > ?", time.Now().Unix()-RelaySessionIdleSeconds)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("(session_key LIKE ? OR username LIKE ? OR token_name LIKE ? OR model_name LIKE ? OR last_group LIKE ? OR override_group LIKE ?)", pattern, pattern, pattern, pattern, pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	sessions := make([]RelaySession, 0)
	err := query.Order("last_seen_at DESC").Order("id").Offset(offset).Limit(limit).Find(&sessions).Error
	return sessions, total, err
}

func UpdateRelaySessionGroup(id, group string) (bool, error) {
	result := DB.Model(&RelaySession{}).
		Where("id = ? AND last_seen_at > ?", id, time.Now().Unix()-RelaySessionIdleSeconds).
		Update("override_group", group)
	// MySQL may report zero affected rows when the value is already identical.
	if result.Error == nil && result.RowsAffected == 0 {
		session, err := GetActiveRelaySession(id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return err == nil && session.OverrideGroup == group, err
	}
	return result.RowsAffected > 0, result.Error
}

func RecordRelaySessionGroup(id, group string, startedAt int64) error {
	return DB.Model(&RelaySession{}).Where("id = ? AND last_seen_at <= ?", id, startedAt).
		Update("last_group", group).Error
}

func DeleteExpiredRelaySessions() error {
	return DB.Where("last_seen_at <= ?", time.Now().Unix()-RelaySessionIdleSeconds).
		Delete(&RelaySession{}).Error
}
