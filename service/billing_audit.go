package service

import (
	"errors"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

const BillingAuditTimezone = "Asia/Shanghai"

var billingAmountPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,2})?$`)

func BillingAuditMonth(month string) (string, int64, int64, error) {
	location, err := time.LoadLocation(BillingAuditTimezone)
	if err != nil {
		return "", 0, 0, err
	}
	if month == "" {
		month = time.Now().In(location).Format("2006-01")
	}
	start, err := time.ParseInLocation("2006-01", month, location)
	if err != nil || start.Format("2006-01") != month || start.Year() < 2000 || start.Year() > 9998 {
		return "", 0, 0, errors.New("invalid month (YYYY-MM, 2000-9998)")
	}
	return month, start.Unix(), start.AddDate(0, 1, 0).Unix(), nil
}

type BillingCostInput struct {
	Month     string `json:"month"`
	Name      string `json:"name"`
	Amount    string `json:"amount"`
	Remark    string `json:"remark"`
	Recurring bool   `json:"recurring"`
	Scope     string `json:"scope"`
}

func SaveBillingCost(id, actor int, input BillingCostInput, remove bool) error {
	if input.Month == "" {
		return errors.New("month is required")
	}
	month, _, _, err := BillingAuditMonth(input.Month)
	if err != nil {
		return err
	}
	name, remark := strings.TrimSpace(input.Name), strings.TrimSpace(input.Remark)
	var cents int64
	if !remove {
		if name == "" || utf8.RuneCountInString(name) > 128 || utf8.RuneCountInString(remark) > 1000 {
			return errors.New("invalid cost name or remark")
		}
		if !billingAmountPattern.MatchString(input.Amount) {
			return errors.New("amount must be positive USD with at most two decimal places")
		}
		amount, err := decimal.NewFromString(input.Amount)
		if err != nil || !amount.IsPositive() || amount.GreaterThan(decimal.NewFromInt(1000000000)) {
			return errors.New("amount must be between 0.01 and 1000000000 USD")
		}
		cents = amount.Mul(decimal.NewFromInt(100)).IntPart()
	}
	if id == 0 {
		if remove {
			return errors.New("cost id is required")
		}
		return model.CreateBillingCost(month, name, remark, cents, input.Recurring, actor)
	}
	return model.ChangeBillingCost(id, month, input.Scope, name, remark, cents, remove, actor)
}

type BillingAuditGroup struct {
	Group         string `json:"group"`
	EstimatedCost string `json:"estimated_cost"`
}

type BillingAuditSummary struct {
	Month          string                 `json:"month"`
	Currency       string                 `json:"currency"`
	Timezone       string                 `json:"timezone"`
	QueriedAt      int64                  `json:"queried_at"`
	RechargeAmount string                 `json:"recharge_amount"`
	RefundAmount   string                 `json:"refund_amount"`
	NetRecharge    string                 `json:"net_recharge"`
	EstimatedCost  string                 `json:"estimated_cost"`
	ActualCost     string                 `json:"actual_cost"`
	TotalCost      string                 `json:"total_cost"`
	CurrentBalance string                 `json:"current_balance"`
	Groups         []BillingAuditGroup    `json:"groups"`
	Costs          []model.BillingCostRow `json:"costs"`
}

func GetBillingAuditSummary(month string) (*BillingAuditSummary, error) {
	month, start, end, err := BillingAuditMonth(month)
	if err != nil {
		return nil, err
	}
	if common.QuotaPerUnit <= 0 {
		return nil, errors.New("invalid quota per unit")
	}
	recharge, err := model.GetBillingRechargeTotals(start, end)
	if err != nil {
		return nil, err
	}
	groups, err := model.GetBillingGroupQuotas(start, end)
	if err != nil {
		return nil, err
	}
	balance, err := model.GetBillingBalanceQuota()
	if err != nil {
		return nil, err
	}
	costs, err := model.GetBillingCosts(month)
	if err != nil {
		return nil, err
	}
	unit := decimal.NewFromFloat(common.QuotaPerUnit)
	result := &BillingAuditSummary{Month: month, Currency: "USD", Timezone: BillingAuditTimezone, QueriedAt: time.Now().Unix(), Groups: make([]BillingAuditGroup, 0, len(groups)), Costs: costs}
	refund := decimal.NewFromInt(recharge.RefundedCents).Div(decimal.NewFromInt(100))
	result.RechargeAmount, result.RefundAmount, result.NetRecharge = recharge.Received.StringFixed(2), refund.StringFixed(2), recharge.Received.Sub(refund).StringFixed(2)
	estimate := decimal.Zero
	for _, group := range groups {
		amount := decimal.NewFromInt(group.Quota).Div(unit)
		estimate = estimate.Add(amount)
		result.Groups = append(result.Groups, BillingAuditGroup{Group: group.Group, EstimatedCost: amount.String()})
	}
	actual := decimal.Zero
	for _, cost := range costs {
		actual = actual.Add(decimal.NewFromInt(cost.AmountCents).Div(decimal.NewFromInt(100)))
	}
	result.EstimatedCost = estimate.String()
	result.ActualCost, result.TotalCost = actual.StringFixed(2), actual.StringFixed(2)
	result.CurrentBalance = decimal.NewFromInt(balance).Div(unit).String()
	return result, nil
}
