package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingManualAmounts(t *testing.T) {
	for _, tc := range []struct {
		name, content, other, amount string
	}{
		{"legacy USD", "＄12.345678 额度", "", "12.345678"},
		{"stored USD wins over display currency", "¥70.000000 额度", `{"admin_info":{"amount_usd":"10.000002"}}`, "10.000002"},
		{"legacy CNY stays in original units", "¥70.000000 额度", "", ""},
		{"legacy tokens stay in original units", "500000 点额度", "", ""},
		{"malformed amount stays visible", "unknown", `{"admin_info":{"amount_usd":"bad"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := billingManualRow(Log{Id: 7, UserId: 9, Type: LogTypeRefund, Content: billingManualRefundPrefix + tc.content, Other: tc.other})
			require.Equal(t, -7, row.ID)
			require.Equal(t, "admin_refund", row.PaymentMethod)
			require.Equal(t, tc.content, row.OriginalAmount)
			if tc.amount == "" {
				require.Nil(t, row.ManualAmount)
			} else {
				require.NotNil(t, row.ManualAmount)
				require.Equal(t, tc.amount, *row.ManualAmount)
			}
		})
	}
}

func setupBillingTopUpTest(t *testing.T) {
	t.Helper()
	setupBillingCostTest(t)
	require.NoError(t, DB.AutoMigrate(&TopUp{}, &User{}))
	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := logDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previous := LOG_DB
	LOG_DB = logDB
	t.Cleanup(func() { LOG_DB = previous; _ = sqlDB.Close() })
	require.NoError(t, logDB.AutoMigrate(&Log{}))
}

func TestBillingTopUpsMergeSeparateDatabases(t *testing.T) {
	setupBillingTopUpTest(t)
	require.NoError(t, DB.Create(&[]TopUp{
		{Id: 1, TradeNo: "order-1", CompleteTime: 13710, Money: 10, PaymentMethod: PaymentMethodXznPay, Status: common.TopUpStatusSuccess},
		{Id: 2, TradeNo: "order-2", CompleteTime: 100120, Money: 20, Status: common.TopUpStatusPartialRefund, ProviderRefundedAmount: 500},
	}).Error)
	require.NoError(t, LOG_DB.Create(&[]Log{
		{Id: 1, Type: LogTypeTopup, CreatedAt: 100100, Content: billingManualRechargePrefix + "＄1.000000 额度"},
		{Id: 2, Type: LogTypeRefund, CreatedAt: 100120, Content: billingManualRefundPrefix + "＄2.000000 额度"},
		{Id: 3, Type: LogTypeTopup, CreatedAt: 100120, Content: billingManualRechargePrefix + "＄3.000000 额度"},
		{Id: 4, Type: LogTypeManage, CreatedAt: 100125, Content: "管理员增加用户额度 ＄999.000000 额度"},
		{Id: 5, Type: LogTypeTopup, CreatedAt: 100125, Content: "管理员补单成功，充值金额: ＄20.00"},
		{Id: 6, Type: LogTypeTopup, CreatedAt: 100200, Content: billingManualRechargePrefix + "＄999.000000 额度"},
		{Id: 7, Type: LogTypeRefund, CreatedAt: 100099, Content: billingManualRefundPrefix + "＄999.000000 额度"},
	}).Error)
	var ids []int
	for page := 1; page <= 4; page++ {
		rows, total, err := GetBillingTopUps(100100, 100200, page, 2)
		require.NoError(t, err)
		require.Equal(t, int64(5), total)
		for _, row := range rows {
			ids = append(ids, row.ID)
			if row.ID == 1 {
				require.Equal(t, 9.7, row.Money)
			}
			if row.ID == 2 {
				require.Equal(t, float64(20), row.Money)
				require.Equal(t, int64(500), row.ProviderRefundedAmount)
			}
		}
	}
	require.Equal(t, []int{2, -3, -2, 1, -1}, ids)
	var original TopUp
	require.NoError(t, DB.First(&original, 1).Error)
	require.Equal(t, float64(10), original.Money)
}
