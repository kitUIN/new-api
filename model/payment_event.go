package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const PaymentProviderXznPay = "xzn_pay"

type PaymentEvent struct {
	Id              int    `json:"id"`
	Provider        string `json:"provider" gorm:"type:varchar(50);uniqueIndex:idx_payment_event_provider_key"`
	EventKey        string `json:"event_key" gorm:"type:varchar(128);uniqueIndex:idx_payment_event_provider_key"`
	TradeNo         string `json:"trade_no" gorm:"type:varchar(255);index"`
	ProviderTradeNo string `json:"provider_trade_no" gorm:"type:varchar(255);index"`
	Status          string `json:"status" gorm:"type:varchar(50)"`
	AmountCents     int64  `json:"amount_cents"`
	QuotaDelta      int64  `json:"quota_delta"`
	EventTime       int64  `json:"event_time"`
	Payload         string `json:"payload" gorm:"type:text"`
	CreateTime      int64  `json:"create_time"`
}

type XznPayEventInput struct {
	EventKey          string
	TradeNo           string
	ProviderTradeNo   string
	ProviderStatus    string
	RefundAmountCents int64
	EventTime         int64
	Payload           string
}

type XznPayEventResult struct {
	Duplicate  bool
	UserID     int
	QuotaDelta int64
	Status     string
}

func ApplyXznPayEvent(input XznPayEventInput, callerIP string) (*XznPayEventResult, error) {
	if input.EventKey == "" || input.TradeNo == "" || input.ProviderStatus == "" {
		return nil, errors.New("XznPay 回调事件参数不完整")
	}

	result := &XznPayEventResult{}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", input.TradeNo).First(topUp).Error; err != nil {
			return ErrTopUpNotFound
		}
		if topUp.PaymentMethod != PaymentMethodXznPay {
			return ErrPaymentMethodMismatch
		}

		var eventCount int64
		if err := tx.Model(&PaymentEvent{}).Where("provider = ? AND event_key = ?", PaymentProviderXznPay, input.EventKey).Count(&eventCount).Error; err != nil {
			return err
		}
		if eventCount > 0 {
			result.Duplicate = true
			result.UserID = topUp.UserId
			result.Status = topUp.Status
			return nil
		}

		balanceDelta, err := applyXznPayState(topUp, input)
		if err != nil {
			return err
		}
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		if balanceDelta != 0 {
			if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", balanceDelta)).Error; err != nil {
				return err
			}
		}

		event := &PaymentEvent{
			Provider:        PaymentProviderXznPay,
			EventKey:        input.EventKey,
			TradeNo:         input.TradeNo,
			ProviderTradeNo: input.ProviderTradeNo,
			Status:          input.ProviderStatus,
			AmountCents:     input.RefundAmountCents,
			QuotaDelta:      balanceDelta,
			EventTime:       input.EventTime,
			Payload:         input.Payload,
			CreateTime:      common.GetTimestamp(),
		}
		if err := tx.Create(event).Error; err != nil {
			return err
		}

		result.UserID = topUp.UserId
		result.QuotaDelta = balanceDelta
		result.Status = topUp.Status
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.QuotaDelta != 0 {
		_ = InvalidateUserCache(result.UserID)
	}
	if !result.Duplicate {
		message := fmt.Sprintf("XznPay 状态更新: %s，额度变动: %s", input.ProviderStatus, logger.FormatQuota(int(result.QuotaDelta)))
		if input.ProviderStatus == "TRADE_SUCCESS" {
			message = fmt.Sprintf("自助充值，额度变动: +%s", logger.FormatQuota(int(result.QuotaDelta)))
		}
		RecordTopupLog(result.UserID, message, callerIP, PaymentMethodXznPay, PaymentMethodXznPay)
	}
	return result, nil
}

func applyXznPayState(topUp *TopUp, input XznPayEventInput) (int64, error) {
	if input.ProviderTradeNo != "" {
		if topUp.ProviderTradeNo != "" && topUp.ProviderTradeNo != input.ProviderTradeNo {
			return 0, errors.New("XznPay 平台订单号不匹配")
		}
		topUp.ProviderTradeNo = input.ProviderTradeNo
	}
	topUp.ProviderStatus = input.ProviderStatus
	balanceDelta := int64(0)

	switch input.ProviderStatus {
	case "WAIT_BUYER_PAY":
		// 保持本地待支付状态。
	case "TRADE_SUCCESS":
		if topUp.CreditedQuota == 0 {
			creditedQuota := decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
			if creditedQuota <= 0 {
				return 0, errors.New("无效的充值额度")
			}
			topUp.CreditedQuota = creditedQuota
			topUp.RefundedQuota = calculateXznPayRefundedQuota(topUp)
			netQuota := topUp.CreditedQuota - topUp.RefundedQuota
			if topUp.ProviderFrozen {
				topUp.FrozenQuota = netQuota
			} else {
				balanceDelta += netQuota
			}
			topUp.CompleteTime = common.GetTimestamp()
		}
	case "TRADE_CLOSED":
		if topUp.CreditedQuota == 0 {
			topUp.Status = common.TopUpStatusFailed
			return balanceDelta, nil
		}
	case "TRADE_FREEZE":
		if !topUp.ProviderFrozen {
			topUp.ProviderFrozen = true
			remainingQuota := topUp.CreditedQuota - topUp.RefundedQuota
			if remainingQuota > 0 {
				topUp.FrozenQuota = remainingQuota
				balanceDelta -= remainingQuota
			}
		}
	case "TRADE_UNFREEZE":
		if topUp.ProviderFrozen {
			balanceDelta += topUp.FrozenQuota
			topUp.FrozenQuota = 0
			topUp.ProviderFrozen = false
		}
	case "TRADE_REFUND":
		if input.RefundAmountCents <= 0 {
			return 0, errors.New("XznPay 退款金额无效")
		}
		paidCents := decimal.NewFromFloat(topUp.Money).Round(2).Shift(2).IntPart()
		if paidCents <= 0 {
			return 0, errors.New("XznPay 原支付金额无效")
		}
		newRefundedAmount := topUp.ProviderRefundedAmount + input.RefundAmountCents
		if newRefundedAmount > paidCents {
			newRefundedAmount = paidCents
		}
		topUp.ProviderRefundedAmount = newRefundedAmount
		newRefundedQuota := calculateXznPayRefundedQuota(topUp)
		quotaToRefund := newRefundedQuota - topUp.RefundedQuota
		if quotaToRefund > 0 {
			if topUp.ProviderFrozen {
				if quotaToRefund > topUp.FrozenQuota {
					quotaToRefund = topUp.FrozenQuota
				}
				topUp.FrozenQuota -= quotaToRefund
			} else {
				balanceDelta -= quotaToRefund
			}
			topUp.RefundedQuota = newRefundedQuota
		}
	default:
		return 0, fmt.Errorf("不支持的 XznPay 状态: %s", input.ProviderStatus)
	}

	topUp.Status = deriveXznPayTopUpStatus(topUp)
	return balanceDelta, nil
}

func calculateXznPayRefundedQuota(topUp *TopUp) int64 {
	if topUp.CreditedQuota <= 0 || topUp.ProviderRefundedAmount <= 0 {
		return 0
	}
	paidCents := decimal.NewFromFloat(topUp.Money).Round(2).Shift(2).IntPart()
	if paidCents <= 0 {
		return 0
	}
	if topUp.ProviderRefundedAmount >= paidCents {
		return topUp.CreditedQuota
	}
	return decimal.NewFromInt(topUp.CreditedQuota).
		Mul(decimal.NewFromInt(topUp.ProviderRefundedAmount)).
		Div(decimal.NewFromInt(paidCents)).
		Floor().IntPart()
}

func deriveXznPayTopUpStatus(topUp *TopUp) string {
	if topUp.CreditedQuota == 0 {
		if topUp.Status == common.TopUpStatusFailed {
			return common.TopUpStatusFailed
		}
		if topUp.ProviderFrozen {
			return common.TopUpStatusFrozen
		}
		return common.TopUpStatusPending
	}
	if topUp.RefundedQuota >= topUp.CreditedQuota {
		return common.TopUpStatusRefunded
	}
	if topUp.ProviderFrozen {
		return common.TopUpStatusFrozen
	}
	if topUp.RefundedQuota > 0 {
		return common.TopUpStatusPartialRefund
	}
	return common.TopUpStatusSuccess
}
