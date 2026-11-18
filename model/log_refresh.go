package model

import "errors"

const MaxLogRefreshIDs = 1000

// GetLogsByIDs refreshes pinned rows even after newer logs move them off a page.
// A positive userID always restricts the result to that user's logs; zero is
// reserved for the admin-authenticated endpoint.
func GetLogsByIDs(userID int, ids []int) ([]*Log, error) {
	if userID < 0 || len(ids) == 0 || len(ids) > MaxLogRefreshIDs {
		return nil, errors.New("invalid log refresh request")
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, errors.New("invalid log ID")
		}
	}
	tx := LOG_DB.Where("id IN ?", ids)
	if userID > 0 {
		tx = tx.Where("user_id = ?", userID)
	}
	logs := make([]*Log, 0, len(ids))
	if err := tx.Order("id desc").Find(&logs).Error; err != nil {
		return nil, err
	}
	if userID == 0 {
		if err := fillLogChannelNames(logs); err != nil {
			return nil, err
		}
	}
	if err := fillLogQQIds(logs); err != nil {
		return nil, err
	}
	return logs, nil
}
