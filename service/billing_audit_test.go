package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingAuditMonthBoundaries(t *testing.T) {
	_, start, end, err := BillingAuditMonth("2026-12")
	require.NoError(t, err)
	require.Equal(t, "2026-11-30T16:00:00Z", time.Unix(start, 0).UTC().Format(time.RFC3339))
	require.Equal(t, "2026-12-31T16:00:00Z", time.Unix(end, 0).UTC().Format(time.RFC3339))
	_, start, end, err = BillingAuditMonth("2028-02")
	require.NoError(t, err)
	require.Equal(t, int64(29*86400), end-start)
	for _, month := range []string{"2026-1", "2026-00", "2026-13", "2026-09-01", "1999-12", "9999-01", "bad"} {
		_, _, _, err := BillingAuditMonth(month)
		require.Error(t, err, month)
	}
}

func TestBillingAuditSummarySeparatesEstimatedAndActual(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldLogDB, oldUnit := model.DB, model.LOG_DB, common.QuotaPerUnit
	model.DB, common.QuotaPerUnit = db, 500000
	model.LOG_DB = db
	t.Cleanup(func() { model.DB, model.LOG_DB, common.QuotaPerUnit = oldDB, oldLogDB, oldUnit; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.User{}, &model.TopUp{}, &model.QuotaData{}, &model.BillingCost{}, &model.BillingCostVersion{}, &model.BillingCostException{}))
	_, start, end, err := BillingAuditMonth("2026-09")
	require.NoError(t, err)
	require.NoError(t, db.Create(&[]model.QuotaData{
		{Group: "a", CreatedAt: start, Quota: 30000000}, {Group: "b", CreatedAt: end - 3600, Quota: 20000000},
		{Group: "a", CreatedAt: start - 3600, Quota: 99999999}, {Group: "b", CreatedAt: end, Quota: 99999999},
	}).Error)
	require.NoError(t, model.CreateBillingCost("2026-09", "Single", "", 2000, false, 1))
	require.NoError(t, model.CreateBillingCost("2026-09", "Recurring", "", 3000, true, 1))
	require.NoError(t, db.Create(&[]model.User{
		{Username: "active", AffCode: "audit-active", Quota: 10000000},
		{Username: "disabled", AffCode: "audit-disabled", Quota: -500000, Status: common.UserStatusDisabled},
		{Username: "deleted", AffCode: "audit-deleted", Quota: 99999999, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}).Error)
	require.NoError(t, db.Create(&[]model.TopUp{
		{TradeNo: "success", Money: 100, PaymentMethod: model.PaymentMethodXznPay, CompleteTime: start, Status: common.TopUpStatusSuccess},
		{TradeNo: "partial", Money: 50, CompleteTime: end - 1, Status: common.TopUpStatusPartialRefund, ProviderRefundedAmount: 1000},
		{TradeNo: "refunded", Money: 20, CompleteTime: start, Status: common.TopUpStatusRefunded, ProviderRefundedAmount: 2000},
		{TradeNo: "frozen", Money: 10, CompleteTime: start, Status: common.TopUpStatusFrozen},
		{TradeNo: "pending", Money: 999, CompleteTime: start, Status: common.TopUpStatusPending},
		{TradeNo: "failed", Money: 999, CompleteTime: start, Status: common.TopUpStatusFailed},
		{TradeNo: "next", Money: 999, CompleteTime: end, Status: common.TopUpStatusSuccess},
		{TradeNo: "previous", Money: 999, CompleteTime: start - 1, Status: common.TopUpStatusSuccess},
	}).Error)
	result, err := GetBillingAuditSummary("2026-09")
	require.NoError(t, err)
	require.Equal(t, "100", result.EstimatedCost)
	require.Equal(t, "50.00", result.ActualCost)
	require.Equal(t, result.ActualCost, result.TotalCost)
	require.Equal(t, "19", result.CurrentBalance)
	require.Equal(t, "177.00", result.RechargeAmount)
	require.Equal(t, "30.00", result.RefundAmount)
	require.Equal(t, "147.00", result.NetRecharge)
	require.Len(t, result.Groups, 2)
	items, total, err := model.GetBillingTopUps(start, end, 1, 2)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, items, 2)
	require.Equal(t, "partial", items[0].TradeNo)
	require.Equal(t, int64(1000), items[0].ProviderRefundedAmount)
	result, err = GetBillingAuditSummary("2026-10")
	require.NoError(t, err)
	require.Equal(t, "199.999998", result.EstimatedCost)
	require.Equal(t, "30.00", result.ActualCost)
	require.Equal(t, "19", result.CurrentBalance)
	require.Equal(t, "999.00", result.RechargeAmount)
	result, err = GetBillingAuditSummary("2026-11")
	require.NoError(t, err)
	require.Equal(t, "0", result.EstimatedCost)
	require.Equal(t, "0.00", result.RechargeAmount)
	// Subscription accrual is included only in actual/total costs, month by month.
	require.NoError(t, SaveBillingCost(0, 1, BillingCostInput{Month: "2026-10", Name: "Subscription", Amount: "1400", Allocation: model.BillingAllocationSubscription, StartDate: "2026-10-07", EndDate: "2026-11-07"}, false))
	result, err = GetBillingAuditSummary("2026-10")
	require.NoError(t, err)
	require.Equal(t, "1113.87", result.ActualCost)
	require.Equal(t, result.ActualCost, result.TotalCost)
	require.Equal(t, "199.999998", result.EstimatedCost)
	result, err = GetBillingAuditSummary("2026-11")
	require.NoError(t, err)
	require.Equal(t, "346.13", result.ActualCost)
	require.Equal(t, "0", result.EstimatedCost)
}

func TestBillingAuditCostInputValidation(t *testing.T) {
	for _, amount := range []string{"0", "-1", "NaN", "Infinity", "1.001", "1e3", "1000000000.01", "", " 1"} {
		err := SaveBillingCost(0, 1, BillingCostInput{Month: "2026-09", Name: "Cost", Amount: amount}, false)
		require.Error(t, err, amount)
	}
	require.Error(t, SaveBillingCost(0, 1, BillingCostInput{Month: "", Name: "Cost", Amount: "1"}, false))
	require.Error(t, SaveBillingCost(0, 1, BillingCostInput{Month: "2026-09", Name: " ", Amount: "1"}, false))
}
