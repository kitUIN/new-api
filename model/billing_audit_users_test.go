package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingUserTopUpsAggregateBeforePagination(t *testing.T) {
	setupBillingTopUpTest(t)
	require.NoError(t, DB.Create(&[]User{
		{Id: 1, Username: "alice", DisplayName: "Alice", QQId: "12345", AffCode: "alice-code"},
		{Id: 2, Username: "deleted", DisplayName: "Former user", AffCode: "deleted-code", DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}).Error)
	require.NoError(t, DB.Create(&[]TopUp{
		{UserId: 1, TradeNo: "alice-xzn", PaymentMethod: PaymentMethodXznPay, Money: 100, CompleteTime: 13700, Status: common.TopUpStatusPartialRefund, ProviderRefundedAmount: 1000},
		{UserId: 1, TradeNo: "alice-other", PaymentMethod: PaymentMethodStripe, Money: 20, CompleteTime: 100110, Status: common.TopUpStatusSuccess},
		{UserId: 2, TradeNo: "deleted-user", Money: 150, CompleteTime: 100110, Status: common.TopUpStatusSuccess},
		{UserId: 1, TradeNo: "outside", Money: 999, CompleteTime: 100200, Status: common.TopUpStatusSuccess},
		{UserId: 1, TradeNo: "pending", Money: 999, CompleteTime: 100120, Status: common.TopUpStatusPending},
	}).Error)
	require.NoError(t, LOG_DB.Create(&[]Log{
		{UserId: 1, Type: LogTypeTopup, CreatedAt: 100100, Content: billingManualRechargePrefix + "＄5.000000 额度"},
		{UserId: 1, Type: LogTypeRefund, CreatedAt: 100120, Content: billingManualRefundPrefix + "¥14.000000 额度", Other: `{"admin_info":{"amount_usd":"2.000002"}}`},
		{UserId: 1, Type: LogTypeRefund, CreatedAt: 100130, Content: billingManualRefundPrefix + "¥7.000000 额度"},
		{UserId: 1, Type: LogTypeManage, CreatedAt: 100130, Content: "管理员增加用户额度 ＄999.000000 额度"},
		{UserId: 3, Type: LogTypeRefund, CreatedAt: 100130, Content: billingManualRefundPrefix + "＄3.000000 额度"},
		{UserId: 1, Type: LogTypeTopup, CreatedAt: 100200, Content: billingManualRechargePrefix + "＄999.000000 额度"},
	}).Error)
	rows, total, err := GetBillingUserTopUps(100100, 100200, 1, 1)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, rows, 1)
	require.Equal(t, 2, rows[0].UserID)
	require.Equal(t, "deleted", rows[0].User.Username)
	require.Equal(t, "150", rows[0].Received.String())
	rows, _, err = GetBillingUserTopUps(100100, 100200, 2, 1)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, BillingAuditUser{Username: "alice", DisplayName: "Alice", QQID: "12345"}, rows[0].User)
	require.Equal(t, "122", rows[0].Received.String())
	require.Equal(t, "12.000002", rows[0].Refunded.String())
	require.Equal(t, 1, rows[0].UnconvertedCount)
	rows, _, err = GetBillingUserTopUps(100100, 100200, 3, 1)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 3, rows[0].UserID)
	require.Empty(t, rows[0].User.Username)
	require.Equal(t, "0", rows[0].Received.String())
	require.Equal(t, "3", rows[0].Refunded.String())
	rows, total, err = GetBillingUserTopUps(100100, 100200, 4, 1)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Empty(t, rows)
	rows, total, err = GetBillingUserTopUps(100201, 100300, 1, 20)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, rows)
	orders, _, err := GetBillingTopUps(100100, 100200, 1, 20)
	require.NoError(t, err)
	for _, order := range orders {
		if order.UserID == 1 {
			require.Equal(t, "alice", order.User.Username)
			require.Equal(t, "12345", order.User.QQID)
		}
	}
}

func TestBillingUserTopUpsManualBatchBoundary(t *testing.T) {
	setupBillingTopUpTest(t)
	logs := make([]Log, 501)
	for i := range logs {
		logs[i] = Log{UserId: 1, Type: LogTypeTopup, CreatedAt: 100100, Content: billingManualRechargePrefix + "＄0.010000 额度"}
	}
	require.NoError(t, LOG_DB.CreateInBatches(&logs, 100).Error)
	rows, total, err := GetBillingUserTopUps(100100, 100200, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.Equal(t, "5.01", rows[0].Received.String())
}
