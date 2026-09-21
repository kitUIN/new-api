package model

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

const (
	BillingAllocationMonth        = "month"
	BillingAllocationSubscription = "subscription"
)

// Dates are civil dates in the audit calendar, represented in UTC so subtraction
// measures whole days. The paid interval is (start, end], matching 10/07 -> 11/07
// as 24 October days and 7 November days.
type BillingCostPeriod struct {
	Allocation string
	StartDate  string
	EndDate    string
}

func billingDate(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Format("2006-01-02") != value || date.Year() < 2000 || date.Year() > 9998 {
		return time.Time{}, errors.New("invalid subscription date (YYYY-MM-DD, 2000-9998)")
	}
	return date, nil
}

// Anchor to the original billing day, rather than allowing February to move all
// subsequent renewals from the 31st to the 28th.
func billingAnniversary(month time.Time, day int) time.Time {
	lastDay := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(month.Year(), month.Month(), day, 0, 0, 0, 0, time.UTC)
}

func NormalizeBillingCostPeriod(period BillingCostPeriod, recurring bool) (BillingCostPeriod, error) {
	if period.Allocation == "" {
		period.Allocation = BillingAllocationMonth
	}
	if period.Allocation == BillingAllocationMonth {
		if period.StartDate != "" || period.EndDate != "" {
			return period, errors.New("subscription dates require subscription allocation")
		}
		return period, nil
	}
	if period.Allocation != BillingAllocationSubscription {
		return period, errors.New("invalid allocation method")
	}
	start, err := billingDate(period.StartDate)
	if err != nil {
		return period, err
	}
	month := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	anniversary := billingAnniversary(month.AddDate(0, 1, 0), start.Day()).Format("2006-01-02")
	if period.EndDate == "" {
		period.EndDate = anniversary
	}
	end, err := billingDate(period.EndDate)
	if err != nil {
		return period, err
	}
	if !end.After(start) {
		return period, errors.New("subscription end must be after start")
	}
	if recurring && period.EndDate != anniversary {
		return period, errors.New("monthly subscriptions must end on the next monthly anniversary")
	}
	return period, nil
}

func billingSubscriptionDates(cost BillingCost, cycleMonth string) (time.Time, time.Time, error) {
	start, err := billingDate(cost.StartDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !cost.Recurring {
		end, err := billingDate(cost.EndDate)
		return start, end, err
	}
	month, err := time.Parse("2006-01", cycleMonth)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return billingAnniversary(month, start.Day()), billingAnniversary(month.AddDate(0, 1, 0), start.Day()), nil
}

// Round cumulative allocations, then subtract them. All monthly pieces sum to
// exactly the paid cents, including multi-month subscriptions and tiny amounts.
func prorateBillingCost(cents int64, start, end, month time.Time) (int64, int, int) {
	firstDay, afterLastDay := start.AddDate(0, 0, 1), end.AddDate(0, 0, 1)
	from, to := month, month.AddDate(0, 1, 0)
	if from.Before(firstDay) {
		from = firstDay
	}
	if to.After(afterLastDay) {
		to = afterLastDay
	}
	totalDays := int((end.Unix() - start.Unix()) / 86400)
	if totalDays <= 0 || !to.After(from) {
		return 0, 0, totalDays
	}
	before := (from.Unix() - firstDay.Unix()) / 86400
	after := (to.Unix() - firstDay.Unix()) / 86400
	amount := decimal.NewFromInt(cents)
	denominator := decimal.NewFromInt(int64(totalDays))
	allocatedBefore := amount.Mul(decimal.NewFromInt(before)).DivRound(denominator, 0)
	allocatedAfter := amount.Mul(decimal.NewFromInt(after)).DivRound(denominator, 0)
	return allocatedAfter.Sub(allocatedBefore).IntPart(), int(after - before), totalDays
}

func resolveBillingCostRow(cost BillingCost, cycleMonth, reportMonth string, versions []BillingCostVersion, exceptions map[string]BillingCostException) (*BillingCostRow, error) {
	if cycleMonth < cost.StartMonth {
		return nil, nil
	}
	var version *BillingCostVersion
	for index := range versions {
		if versions[index].Month <= cycleMonth {
			version = &versions[index]
		}
	}
	if version == nil || version.Disabled {
		return nil, nil
	}
	row := &BillingCostRow{ID: cost.ID, Month: reportMonth, CycleMonth: cycleMonth, StartMonth: cost.StartMonth, Recurring: cost.Recurring, Allocation: cost.Allocation, Name: version.Name, AmountCents: version.AmountCents, Remark: version.Remark}
	if row.Allocation == "" {
		row.Allocation = BillingAllocationMonth
	}
	if exception, ok := exceptions[cycleMonth]; ok {
		if exception.Disabled {
			return nil, nil
		}
		row.Exception, row.Name, row.AmountCents, row.Remark = true, exception.Name, exception.AmountCents, exception.Remark
	}
	row.PeriodAmountCents = row.AmountCents
	if cost.Allocation != BillingAllocationSubscription {
		return row, nil
	}
	start, end, err := billingSubscriptionDates(cost, cycleMonth)
	if err != nil {
		return nil, err
	}
	month, err := time.Parse("2006-01", reportMonth)
	if err != nil {
		return nil, err
	}
	row.AmountCents, row.AllocatedDays, row.PeriodDays = prorateBillingCost(row.AmountCents, start, end, month)
	if row.AllocatedDays == 0 {
		return nil, nil
	}
	row.PeriodStart, row.PeriodEnd = start.Format("2006-01-02"), end.Format("2006-01-02")
	return row, nil
}
