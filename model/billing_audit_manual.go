package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const billingManualRechargePrefix = "管理员充值用户额度 "
const billingManualRefundPrefix = "管理员退款扣减用户额度 "

func billingManualLogs(start, end int64) *gorm.DB {
	return LOG_DB.Model(&Log{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("(type = ? AND content LIKE ?) OR (type = ? AND content LIKE ?)",
			LogTypeTopup, billingManualRechargePrefix+"%", LogTypeRefund, billingManualRefundPrefix+"%")
}

func billingManualRow(log Log) BillingTopUpRow {
	prefix, method := billingManualRechargePrefix, "admin_recharge"
	if log.Type == LogTypeRefund {
		prefix, method = billingManualRefundPrefix, "admin_refund"
	}
	row := BillingTopUpRow{
		ID: -log.Id, UserID: log.UserId, TradeNo: fmt.Sprintf("admin-log-%d", log.Id),
		PaymentMethod: method, CompleteTime: log.CreatedAt, BillingTime: log.CreatedAt, Status: common.TopUpStatusSuccess,
		OriginalAmount: strings.TrimPrefix(log.Content, prefix),
	}
	var other struct {
		AdminInfo struct {
			AmountUSD *string `json:"amount_usd"`
		} `json:"admin_info"`
	}
	if common.UnmarshalJsonStr(log.Other, &other) == nil && other.AdminInfo.AmountUSD != nil {
		amount, err := decimal.NewFromString(*other.AdminInfo.AmountUSD)
		if err == nil && !amount.IsNegative() {
			value := amount.String()
			row.ManualAmount = &value
			return row
		}
	}
	// Older logs contain only a display string. A full-width dollar sign is
	// emitted by LogQuota's USD mode. Preserve other currencies verbatim:
	// their historical exchange rate was not stored and cannot be recovered.
	if strings.HasPrefix(row.OriginalAmount, "＄") && strings.HasSuffix(row.OriginalAmount, " 额度") {
		value := strings.TrimSuffix(strings.TrimPrefix(row.OriginalAmount, "＄"), " 额度")
		amount, err := decimal.NewFromString(value)
		if err == nil && !amount.IsNegative() {
			value = amount.String()
			row.ManualAmount = &value
		}
	}
	return row
}

func billingManualAmount(log Log) (decimal.Decimal, bool) {
	row := billingManualRow(log)
	if row.ManualAmount == nil {
		return decimal.Zero, false
	}
	amount, err := decimal.NewFromString(*row.ManualAmount)
	return amount, err == nil
}
