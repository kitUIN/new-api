package model

import (
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BillingDailyTopUpRow struct {
	Date             string          `json:"date"`
	RecordCount      int64           `json:"record_count"`
	Received         decimal.Decimal `json:"received"`
	Refunded         decimal.Decimal `json:"refunded"`
	NetRecharge      decimal.Decimal `json:"net_recharge"`
	UnconvertedCount int             `json:"unconverted_count"`
}

type billingDailyTopUpGroup struct {
	BillingDate   string
	PaymentMethod string
	RecordCount   int64
	Received      decimal.Decimal
	RefundedCents int64
}

func billingDailyTopUpQuery(db *gorm.DB, start, end int64) *gorm.DB {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	var dateCase strings.Builder
	dateCase.WriteString("CASE ")
	arguments := make([]interface{}, 0)
	for dayStart := start; dayStart < end; dayStart += 86400 {
		date := time.Unix(dayStart, 0).In(location).Format("2006-01-02")
		dateCase.WriteString("WHEN billing_time < ? THEN ? ")
		arguments = append(arguments, min(dayStart+86400, end), date)
	}
	dateCase.WriteString("END")
	paid := billingPaidTopUpsQuery(db, start, end).Select(
		"payment_method, money, provider_refunded_amount, CASE WHEN payment_method = ? THEN complete_time + ? ELSE complete_time END AS billing_time",
		PaymentMethodXznPay, billingXznSettlementDelay,
	)
	return db.Table("(?) AS paid_topups", paid).
		Select(dateCase.String()+" AS billing_date, payment_method, COUNT(*) AS record_count, COALESCE(SUM(money), 0) AS received, COALESCE(SUM(provider_refunded_amount), 0) AS refunded_cents", arguments...).
		Group("billing_date, payment_method")
}

func GetBillingDailyTopUps(start, end int64, page, size int) ([]BillingDailyTopUpRow, int64, error) {
	byDate := make(map[string]*BillingDailyTopUpRow)
	get := func(date string) *BillingDailyTopUpRow {
		if byDate[date] == nil {
			byDate[date] = &BillingDailyTopUpRow{Date: date, Received: decimal.Zero, Refunded: decimal.Zero}
		}
		return byDate[date]
	}
	var groups []billingDailyTopUpGroup
	if err := billingDailyTopUpQuery(DB, start, end).Scan(&groups).Error; err != nil {
		return nil, 0, err
	}
	for _, group := range groups {
		row := get(group.BillingDate)
		row.RecordCount += group.RecordCount
		row.Received = row.Received.Add(billingReceivedAmount(group.Received, group.PaymentMethod))
		row.Refunded = row.Refunded.Add(decimal.NewFromInt(group.RefundedCents).Div(decimal.NewFromInt(100)))
	}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	var logs []Log
	if err := billingManualLogs(start, end).FindInBatches(&logs, 500, func(_ *gorm.DB, _ int) error {
		for _, log := range logs {
			row := get(time.Unix(log.CreatedAt, 0).In(location).Format("2006-01-02"))
			row.RecordCount++
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
	rows := make([]BillingDailyTopUpRow, 0, len(byDate))
	for _, row := range byDate {
		row.NetRecharge = row.Received.Sub(row.Refunded)
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(left, right int) bool { return rows[left].Date > rows[right].Date })
	total := int64(len(rows))
	offset := (page - 1) * size
	if offset >= len(rows) {
		return []BillingDailyTopUpRow{}, total, nil
	}
	return rows[offset:min(offset+size, len(rows))], total, nil
}
