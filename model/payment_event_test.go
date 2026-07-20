package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupXznPayTopUpTest(t *testing.T, userQuota int) *TopUp {
	t.Helper()
	truncateTables(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	user := &User{Id: 901, Username: "xzn_pay_user", Status: common.UserStatusEnabled, Quota: userQuota}
	require.NoError(t, DB.Create(user).Error)
	topUp := &TopUp{
		UserId:          user.Id,
		Amount:          10,
		Money:           10,
		TradeNo:         "XZN-ORDER-1",
		PaymentMethod:   PaymentMethodXznPay,
		ProviderPayType: "alipay",
		ProviderStatus:  "WAIT_BUYER_PAY",
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
	}
	require.NoError(t, topUp.Insert())
	return topUp
}

func applyXznPayTestEvent(t *testing.T, key string, status string, refundCents int64) *XznPayEventResult {
	t.Helper()
	result, err := ApplyXznPayEvent(XznPayEventInput{
		EventKey:          key,
		TradeNo:           "XZN-ORDER-1",
		ProviderTradeNo:   "P-ORDER-1",
		ProviderStatus:    status,
		RefundAmountCents: refundCents,
		EventTime:         common.GetTimestamp(),
		Payload:           fmt.Sprintf(`{"status":%q}`, status),
	}, "127.0.0.1")
	require.NoError(t, err)
	return result
}

func TestApplyXznPayEventLifecycle(t *testing.T) {
	setupXznPayTopUpTest(t, 0)

	success := applyXznPayTestEvent(t, "success-1", "TRADE_SUCCESS", 0)
	assert.EqualValues(t, 1000, success.QuotaDelta)
	assert.Equal(t, 1000, getUserQuotaForPaymentGuardTest(t, 901))
	assert.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, "XZN-ORDER-1"))

	duplicate := applyXznPayTestEvent(t, "success-1", "TRADE_SUCCESS", 0)
	assert.True(t, duplicate.Duplicate)
	assert.Equal(t, 1000, getUserQuotaForPaymentGuardTest(t, 901))

	freeze := applyXznPayTestEvent(t, "freeze-1", "TRADE_FREEZE", 0)
	assert.EqualValues(t, -1000, freeze.QuotaDelta)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 901))
	assert.Equal(t, common.TopUpStatusFrozen, getTopUpStatusForPaymentGuardTest(t, "XZN-ORDER-1"))

	refund := applyXznPayTestEvent(t, "refund-1", "TRADE_REFUND", 500)
	assert.Zero(t, refund.QuotaDelta)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 901))

	unfreeze := applyXznPayTestEvent(t, "unfreeze-1", "TRADE_UNFREEZE", 0)
	assert.EqualValues(t, 500, unfreeze.QuotaDelta)
	assert.Equal(t, 500, getUserQuotaForPaymentGuardTest(t, 901))
	assert.Equal(t, common.TopUpStatusPartialRefund, getTopUpStatusForPaymentGuardTest(t, "XZN-ORDER-1"))

	fullRefund := applyXznPayTestEvent(t, "refund-2", "TRADE_REFUND", 500)
	assert.EqualValues(t, -500, fullRefund.QuotaDelta)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 901))
	assert.Equal(t, common.TopUpStatusRefunded, getTopUpStatusForPaymentGuardTest(t, "XZN-ORDER-1"))
}

func TestApplyXznPayEventAllowsNegativeBalance(t *testing.T) {
	setupXznPayTopUpTest(t, 0)
	applyXznPayTestEvent(t, "success-negative", "TRADE_SUCCESS", 0)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 901).Update("quota", 100).Error)

	refund := applyXznPayTestEvent(t, "refund-negative", "TRADE_REFUND", 1000)
	assert.EqualValues(t, -1000, refund.QuotaDelta)
	assert.Equal(t, -900, getUserQuotaForPaymentGuardTest(t, 901))
}

func TestApplyXznPayEventHandlesFreezeBeforeSuccess(t *testing.T) {
	setupXznPayTopUpTest(t, 0)
	applyXznPayTestEvent(t, "freeze-before-success", "TRADE_FREEZE", 0)
	success := applyXznPayTestEvent(t, "success-after-freeze", "TRADE_SUCCESS", 0)

	assert.Zero(t, success.QuotaDelta)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 901))
	assert.Equal(t, common.TopUpStatusFrozen, getTopUpStatusForPaymentGuardTest(t, "XZN-ORDER-1"))

	unfreeze := applyXznPayTestEvent(t, "unfreeze-after-success", "TRADE_UNFREEZE", 0)
	assert.EqualValues(t, 1000, unfreeze.QuotaDelta)
	assert.Equal(t, 1000, getUserQuotaForPaymentGuardTest(t, 901))
}

func TestManualCompleteXznPayDoesNotDoubleCreditOnSuccessCallback(t *testing.T) {
	setupXznPayTopUpTest(t, 0)
	require.NoError(t, ManualCompleteTopUp("XZN-ORDER-1", "127.0.0.1"))
	assert.Equal(t, 1000, getUserQuotaForPaymentGuardTest(t, 901))

	result := applyXznPayTestEvent(t, "success-after-manual", "TRADE_SUCCESS", 0)
	assert.Zero(t, result.QuotaDelta)
	assert.Equal(t, 1000, getUserQuotaForPaymentGuardTest(t, 901))
}
