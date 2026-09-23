package model

import (
	"sort"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BillingAuditUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	QQID        string `json:"qq_id"`
}

func billingAuditUsers(ids []int) (map[int]BillingAuditUser, error) {
	result := make(map[int]BillingAuditUser)
	if len(ids) == 0 {
		return result, nil
	}
	var users []User
	// Keep historical identity for soft-deleted users, and never expose auth fields.
	if err := DB.Unscoped().Select("id, username, display_name, qq_id").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		result[user.Id] = BillingAuditUser{Username: user.Username, DisplayName: user.DisplayName, QQID: user.QQId}
	}
	return result, nil
}

type BillingUserTopUpRow struct {
	UserID           int              `json:"user_id"`
	User             BillingAuditUser `json:"user"`
	Received         decimal.Decimal  `json:"received"`
	Refunded         decimal.Decimal  `json:"refunded"`
	UnconvertedCount int              `json:"unconverted_count"`
}

// Aggregate the entire month before pagination, including manual operations in
// the separate log database. Unconvertible legacy amounts are explicitly flagged.
func GetBillingUserTopUps(start, end int64, page, size int) ([]BillingUserTopUpRow, int64, error) {
	byUser := make(map[int]*BillingUserTopUpRow)
	get := func(id int) *BillingUserTopUpRow {
		if byUser[id] == nil {
			byUser[id] = &BillingUserTopUpRow{UserID: id, Received: decimal.Zero, Refunded: decimal.Zero}
		}
		return byUser[id]
	}
	var groups []struct {
		UserID        int
		PaymentMethod string
		Received      decimal.Decimal
		RefundedCents int64
	}
	if err := BillingPaidTopUps(start, end).Select("user_id, payment_method, COALESCE(SUM(money), 0) AS received, COALESCE(SUM(provider_refunded_amount), 0) AS refunded_cents").Group("user_id, payment_method").Scan(&groups).Error; err != nil {
		return nil, 0, err
	}
	for _, group := range groups {
		row := get(group.UserID)
		row.Received = row.Received.Add(billingReceivedAmount(group.Received, group.PaymentMethod))
		row.Refunded = row.Refunded.Add(decimal.NewFromInt(group.RefundedCents).Div(decimal.NewFromInt(100)))
	}
	var logs []Log
	if err := billingManualLogs(start, end).FindInBatches(&logs, 500, func(_ *gorm.DB, _ int) error {
		for _, log := range logs {
			row := get(log.UserId)
			amount, ok := billingManualAmount(log)
			if !ok {
				row.UnconvertedCount++
				continue
			}
			if log.Type == LogTypeRefund {
				row.Refunded = row.Refunded.Add(amount)
			} else {
				row.Received = row.Received.Add(amount)
			}
		}
		return nil
	}).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]BillingUserTopUpRow, 0, len(byUser))
	for _, row := range byUser {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Received.Equal(rows[j].Received) {
			return rows[i].UserID < rows[j].UserID
		}
		return rows[i].Received.GreaterThan(rows[j].Received)
	})
	total := int64(len(rows))
	offset := (page - 1) * size
	if offset >= len(rows) {
		return []BillingUserTopUpRow{}, total, nil
	}
	rows = rows[offset:min(offset+size, len(rows))]
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	users, err := billingAuditUsers(ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range rows {
		rows[i].User = users[rows[i].UserID]
	}
	return rows, total, nil
}
