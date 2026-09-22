package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestBillingXznSettlementMonthBoundaries(t *testing.T) {
	for _, month := range []string{"2026-09", "2026-12", "2028-02"} {
		t.Run(month, func(t *testing.T) {
			setupBillingTopUpTest(t)
			date, err := time.ParseInLocation("2006-01", month, time.FixedZone("Asia/Shanghai", 8*3600))
			require.NoError(t, err)
			start, end := date.Unix(), date.AddDate(0, 1, 0).Unix()
			require.NoError(t, DB.Create(&[]TopUp{
				{TradeNo: "before", PaymentMethod: PaymentMethodXznPay, Money: 999, CompleteTime: start - 86401, Status: common.TopUpStatusSuccess},
				{TradeNo: "first", PaymentMethod: PaymentMethodXznPay, Money: 10, CompleteTime: start - 86400, Status: common.TopUpStatusPartialRefund, ProviderRefundedAmount: 100},
				{TradeNo: "last", PaymentMethod: PaymentMethodXznPay, Money: 20, CompleteTime: end - 86401, Status: common.TopUpStatusSuccess},
				{TradeNo: "next", PaymentMethod: PaymentMethodXznPay, Money: 999, CompleteTime: end - 86400, Status: common.TopUpStatusSuccess},
				{TradeNo: "normal", PaymentMethod: PaymentMethodStripe, Money: 5, CompleteTime: end - 2, Status: common.TopUpStatusSuccess},
				{TradeNo: "normal-before", Money: 999, CompleteTime: start - 1, Status: common.TopUpStatusSuccess},
				{TradeNo: "pending", PaymentMethod: PaymentMethodXznPay, Money: 999, CompleteTime: start, Status: common.TopUpStatusPending},
			}).Error)
			var trades []string
			for page := 1; page <= 3; page++ {
				rows, total, err := GetBillingTopUps(start, end, page, 1)
				require.NoError(t, err)
				require.Equal(t, int64(3), total)
				require.Len(t, rows, 1)
				trades = append(trades, rows[0].TradeNo)
				if rows[0].PaymentMethod == PaymentMethodXznPay {
					require.Equal(t, rows[0].CompleteTime+86400, rows[0].BillingTime)
				} else {
					require.Equal(t, rows[0].CompleteTime, rows[0].BillingTime)
				}
			}
			require.Equal(t, []string{"last", "normal", "first"}, trades)
			totals, err := GetBillingRechargeTotals(start, end)
			require.NoError(t, err)
			require.Equal(t, "34.1", totals.Received.String())
			require.Equal(t, int64(100), totals.RefundedCents)
			users, total, err := GetBillingUserTopUps(start, end, 1, 20)
			require.NoError(t, err)
			require.Equal(t, int64(1), total)
			require.Equal(t, "34.1", users[0].Received.String())
			require.Equal(t, "1", users[0].Refunded.String())
		})
	}
}
