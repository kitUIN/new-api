package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestBillingDailyTopUpsAggregateBeforePagination(test *testing.T) {
	setupBillingTopUpTest(test)
	month := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))
	start, end := month.Unix(), month.AddDate(0, 1, 0).Unix()
	require.NoError(test, DB.Create(&[]TopUp{
		{TradeNo: "xzn-first", PaymentMethod: PaymentMethodXznPay, Money: 100, CompleteTime: start - 86400, Status: common.TopUpStatusPartialRefund, ProviderRefundedAmount: 1000},
		{TradeNo: "normal-first", Money: 20, CompleteTime: start, Status: common.TopUpStatusFrozen, ProviderRefundedAmount: 200},
		{TradeNo: "before-midnight", Money: 7, CompleteTime: start + 86399, Status: common.TopUpStatusSuccess},
		{TradeNo: "midnight", Money: 10, CompleteTime: start + 86400, Status: common.TopUpStatusRefunded, ProviderRefundedAmount: 1300},
		{TradeNo: "xzn-last", PaymentMethod: PaymentMethodXznPay, Money: 20, CompleteTime: end - 86401, Status: common.TopUpStatusSuccess},
		{TradeNo: "normal-last", PaymentMethod: PaymentMethodStripe, Money: 5, CompleteTime: end - 1, Status: common.TopUpStatusSuccess},
		{TradeNo: "xzn-before", PaymentMethod: PaymentMethodXznPay, Money: 999, CompleteTime: start - 86401, Status: common.TopUpStatusSuccess},
		{TradeNo: "normal-before", Money: 999, CompleteTime: start - 1, Status: common.TopUpStatusSuccess},
		{TradeNo: "xzn-next", PaymentMethod: PaymentMethodXznPay, Money: 999, CompleteTime: end - 86400, Status: common.TopUpStatusSuccess},
		{TradeNo: "normal-next", Money: 999, CompleteTime: end, Status: common.TopUpStatusSuccess},
		{TradeNo: "pending", Money: 999, CompleteTime: start, Status: common.TopUpStatusPending},
	}).Error)
	require.NoError(test, LOG_DB.Create(&[]Log{
		{Type: LogTypeTopup, CreatedAt: start, Content: billingManualRechargePrefix + "＄5.000000 额度"},
		{Type: LogTypeRefund, CreatedAt: start + 86400, Content: billingManualRefundPrefix + "¥21.000000 额度", Other: `{"admin_info":{"amount_usd":"3.000002"}}`},
		{Type: LogTypeTopup, CreatedAt: start + 86401, Content: billingManualRechargePrefix + "¥7.000000 额度"},
		{Type: LogTypeManage, CreatedAt: start, Content: "管理员增加用户额度 ＄999.000000 额度"},
		{Type: LogTypeTopup, CreatedAt: start, Content: "管理员补单成功，充值金额: ＄999.00"},
		{Type: LogTypeTopup, CreatedAt: start - 1, Content: billingManualRechargePrefix + "＄999.000000 额度"},
		{Type: LogTypeRefund, CreatedAt: end, Content: billingManualRefundPrefix + "＄999.000000 额度"},
	}).Error)
	rows, total, err := GetBillingDailyTopUps(start, end, 1, 20)
	require.NoError(test, err)
	require.Equal(test, int64(3), total)
	require.Len(test, rows, 3)
	for rowIndex, expected := range []struct {
		date, received, refunded, net string
		count                         int64
		unconverted                   int
	}{
		{"2026-09-30", "24.6", "0", "24.6", 2, 0},
		{"2026-09-02", "10", "16.000002", "-6.000002", 3, 1},
		{"2026-09-01", "130", "12", "118", 4, 0},
	} {
		require.Equal(test, expected.date, rows[rowIndex].Date)
		require.Equal(test, expected.received, rows[rowIndex].Received.String())
		require.Equal(test, expected.refunded, rows[rowIndex].Refunded.String())
		require.Equal(test, expected.net, rows[rowIndex].NetRecharge.String())
		require.Equal(test, expected.count, rows[rowIndex].RecordCount)
		require.Equal(test, expected.unconverted, rows[rowIndex].UnconvertedCount)
		pageRows, pageTotal, err := GetBillingDailyTopUps(start, end, rowIndex+1, 1)
		require.NoError(test, err)
		require.Equal(test, total, pageTotal)
		require.Equal(test, rows[rowIndex:rowIndex+1], pageRows)
	}
	totals, err := GetBillingRechargeTotals(start, end)
	require.NoError(test, err)
	received, refunded := decimal.Zero, decimal.Zero
	for _, row := range rows {
		received = received.Add(row.Received)
		refunded = refunded.Add(row.Refunded)
	}
	require.True(test, totals.Received.Equal(received))
	require.True(test, totals.Refunded.Equal(refunded))
	rows, total, err = GetBillingDailyTopUps(start, end, 4, 1)
	require.NoError(test, err)
	require.Equal(test, int64(3), total)
	require.Empty(test, rows)
	rows, total, err = GetBillingDailyTopUps(end+86400, end+2*86400, 1, 20)
	require.NoError(test, err)
	require.Zero(test, total)
	require.NotNil(test, rows)
	require.Empty(test, rows)
}

func TestBillingDailyTopUpsMonthBoundaries(test *testing.T) {
	for _, value := range []string{"2026-12", "2028-02"} {
		test.Run(value, func(test *testing.T) {
			setupBillingTopUpTest(test)
			month, err := time.ParseInLocation("2006-01", value, time.FixedZone("Asia/Shanghai", 8*3600))
			require.NoError(test, err)
			start, end := month.Unix(), month.AddDate(0, 1, 0).Unix()
			require.NoError(test, DB.Create(&[]TopUp{
				{TradeNo: "first", Money: 1, CompleteTime: start, Status: common.TopUpStatusSuccess},
				{TradeNo: "last", PaymentMethod: PaymentMethodXznPay, Money: 10, CompleteTime: end - 86401, Status: common.TopUpStatusSuccess},
			}).Error)
			rows, total, err := GetBillingDailyTopUps(start, end, 1, 20)
			require.NoError(test, err)
			require.Equal(test, int64(2), total)
			require.Equal(test, month.AddDate(0, 1, -1).Format("2006-01-02"), rows[0].Date)
			require.Equal(test, "9.8", rows[0].Received.String())
			require.Equal(test, value+"-01", rows[1].Date)
		})
	}
}

func TestBillingDailyTopUpsBatchBoundary(test *testing.T) {
	setupBillingTopUpTest(test)
	month := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))
	orders := make([]TopUp, 25)
	for orderIndex := range orders {
		orders[orderIndex] = TopUp{TradeNo: fmt.Sprintf("daily-%d", orderIndex), Money: 1, CompleteTime: month.Unix(), Status: common.TopUpStatusSuccess}
	}
	require.NoError(test, DB.Create(&orders).Error)
	logs := make([]Log, 501)
	for logIndex := range logs {
		logs[logIndex] = Log{Type: LogTypeTopup, CreatedAt: month.Unix(), Content: billingManualRechargePrefix + "＄0.010000 额度"}
	}
	require.NoError(test, LOG_DB.CreateInBatches(&logs, 100).Error)
	rows, total, err := GetBillingDailyTopUps(month.Unix(), month.AddDate(0, 1, 0).Unix(), 1, 1)
	require.NoError(test, err)
	require.Equal(test, int64(1), total)
	require.Len(test, rows, 1)
	require.Equal(test, int64(526), rows[0].RecordCount)
	require.Equal(test, "30.01", rows[0].Received.String())
}

func TestBillingDailyTopUpDialectQueries(test *testing.T) {
	for name, dialect := range map[string]gorm.Dialector{
		"sqlite":   sqlite.Open(":memory:"),
		"mysql":    mysql.New(mysql.Config{SkipInitializeWithVersion: true}),
		"postgres": postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}),
	} {
		test.Run(name, func(test *testing.T) {
			db, err := gorm.Open(dialect, &gorm.Config{DisableAutomaticPing: true, DryRun: true, SkipDefaultTransaction: true})
			require.NoError(test, err)
			var groups []billingDailyTopUpGroup
			query := billingDailyTopUpQuery(db, 100000, 100000+31*86400).Find(&groups)
			require.NoError(test, query.Error)
			require.Contains(test, query.Statement.SQL.String(), "GROUP BY billing_date, payment_method")
			require.Contains(test, query.Statement.SQL.String(), "COUNT(*) AS record_count")
			require.Contains(test, query.Statement.SQL.String(), "AS paid_topups")
			require.Contains(test, query.Statement.SQL.String(), "complete_time +")
			require.Contains(test, query.Statement.Vars, PaymentMethodXznPay)
			require.Contains(test, query.Statement.Vars, billingXznSettlementDelay)
		})
	}
}
