package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBillingSubscriptionExampleAndSinglePeriodEdit(t *testing.T) {
	setupBillingCostTest(t)
	require.NoError(t, CreateBillingCostWithPeriod("2026-10", "Account subscription", "", 140000, false, 1, BillingCostPeriod{Allocation: BillingAllocationSubscription, StartDate: "2026-10-07", EndDate: "2026-11-07"}))
	var id int
	for _, test := range []struct {
		month string
		cents int64
		days  int
	}{{"2026-09", 0, 0}, {"2026-10", 108387, 24}, {"2026-11", 31613, 7}, {"2026-12", 0, 0}} {
		rows, err := GetBillingCosts(test.month)
		require.NoError(t, err)
		if test.days == 0 {
			require.Empty(t, rows)
			continue
		}
		require.Len(t, rows, 1)
		id = rows[0].ID
		require.Equal(t, test.cents, rows[0].AmountCents)
		require.Equal(t, test.days, rows[0].AllocatedDays)
		require.Equal(t, 31, rows[0].PeriodDays)
		require.Equal(t, int64(140000), rows[0].PeriodAmountCents)
		require.Equal(t, "2026-10", rows[0].CycleMonth)
		require.Equal(t, "2026-10-07", rows[0].PeriodStart)
		require.Equal(t, "2026-11-07", rows[0].PeriodEnd)
	}
	// An edit from the November tail addresses the October cycle's total, not
	// November's allocated amount. Both pieces must change together.
	require.NoError(t, ChangeBillingCost(id, "2026-10", "month", "Adjusted", "", 31000, false, 2))
	oct, err := GetBillingCosts("2026-10")
	require.NoError(t, err)
	nov, err := GetBillingCosts("2026-11")
	require.NoError(t, err)
	require.Equal(t, int64(24000), oct[0].AmountCents)
	require.Equal(t, int64(7000), nov[0].AmountCents)
	require.NoError(t, ChangeBillingCost(id, "2026-10", "month", "", "", 0, true, 2))
	for _, month := range []string{"2026-10", "2026-11"} {
		rows, err := GetBillingCosts(month)
		require.NoError(t, err)
		require.Empty(t, rows)
	}
}

func TestBillingSubscriptionRenewalVersionAndStopPreservePaidTail(t *testing.T) {
	setupBillingCostTest(t)
	require.NoError(t, CreateBillingCostWithPeriod("2026-10", "Subscription", "", 140000, true, 1, BillingCostPeriod{Allocation: BillingAllocationSubscription, StartDate: "2026-10-07"}))
	rows, err := GetBillingCosts("2026-10")
	require.NoError(t, err)
	id := rows[0].ID
	require.NoError(t, ChangeBillingCost(id, "2026-11", "future", "Renewed", "", 30000, false, 2))
	rows, err = GetBillingCosts("2026-11")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "2026-10", rows[0].CycleMonth)
	require.Equal(t, int64(31613), rows[0].AmountCents)
	require.Equal(t, "2026-11", rows[1].CycleMonth)
	require.Equal(t, int64(23000), rows[1].AmountCents)
	require.Equal(t, int64(30000), rows[1].PeriodAmountCents)
	// A cycle exception also carries its own amount into the following month.
	require.NoError(t, ChangeBillingCost(id, "2026-11", "month", "Special", "", 60000, false, 2))
	rows, err = GetBillingCosts("2026-11")
	require.NoError(t, err)
	require.Equal(t, int64(31613), rows[0].AmountCents)
	require.Equal(t, int64(46000), rows[1].AmountCents)
	require.NoError(t, ChangeBillingCost(id, "2026-12", "future", "", "", 0, true, 2))
	rows, err = GetBillingCosts("2026-12")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "2026-11", rows[0].CycleMonth)
	require.Equal(t, int64(14000), rows[0].AmountCents)
	rows, err = GetBillingCosts("2027-01")
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = GetBillingCosts("2026-10")
	require.NoError(t, err)
	require.Equal(t, int64(108387), rows[0].AmountCents)
}

func TestBillingSubscriptionMonthEndLeapYearAndMultiMonthRounding(t *testing.T) {
	for _, test := range []struct{ start, cycle, expectedStart, expectedEnd string }{
		{"2027-01-31", "2027-01", "2027-01-31", "2027-02-28"},
		{"2027-01-31", "2027-02", "2027-02-28", "2027-03-31"},
		{"2028-01-31", "2028-01", "2028-01-31", "2028-02-29"},
		{"2028-01-31", "2028-02", "2028-02-29", "2028-03-31"},
		{"2026-12-07", "2026-12", "2026-12-07", "2027-01-07"},
	} {
		start, end, err := billingSubscriptionDates(BillingCost{Recurring: true, StartDate: test.start}, test.cycle)
		require.NoError(t, err)
		require.Equal(t, test.expectedStart, start.Format("2006-01-02"))
		require.Equal(t, test.expectedEnd, end.Format("2006-01-02"))
	}
	for _, cents := range []int64{1, 2, 100, 140000, 100000000000} {
		start, _ := billingDate("2027-12-07")
		end, _ := billingDate("2028-04-07")
		var allocated int64
		days := 0
		for _, month := range []string{"2027-12", "2028-01", "2028-02", "2028-03", "2028-04"} {
			date, _ := time.Parse("2006-01", month)
			amount, count, total := prorateBillingCost(cents, start, end, date)
			require.Equal(t, 122, total)
			require.GreaterOrEqual(t, amount, int64(0))
			allocated += amount
			days += count
		}
		require.Equal(t, cents, allocated)
		require.Equal(t, 122, days)
	}
	// A January 31st subscription allocates all its first cycle to February.
	setupBillingCostTest(t)
	require.NoError(t, CreateBillingCostWithPeriod("2028-01", "Month end", "", 2900, true, 1, BillingCostPeriod{Allocation: BillingAllocationSubscription, StartDate: "2028-01-31"}))
	rows, err := GetBillingCosts("2028-01")
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = GetBillingCosts("2028-02")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(2900), rows[0].AmountCents)
	require.Equal(t, 29, rows[0].AllocatedDays)
}

func TestBillingSubscriptionPeriodValidation(t *testing.T) {
	for _, test := range []struct {
		period    BillingCostPeriod
		recurring bool
	}{
		{BillingCostPeriod{Allocation: "unknown"}, false},
		{BillingCostPeriod{Allocation: BillingAllocationMonth, StartDate: "2026-10-07"}, false},
		{BillingCostPeriod{Allocation: BillingAllocationSubscription, StartDate: "2026-02-29"}, false},
		{BillingCostPeriod{Allocation: BillingAllocationSubscription, StartDate: "2026-10-07", EndDate: "2026-10-07"}, false},
		{BillingCostPeriod{Allocation: BillingAllocationSubscription, StartDate: "2026-10-07", EndDate: "2026-10-06"}, false},
		{BillingCostPeriod{Allocation: BillingAllocationSubscription, StartDate: "2026-10-07", EndDate: "2026-12-07"}, true},
	} {
		_, err := NormalizeBillingCostPeriod(test.period, test.recurring)
		require.Error(t, err)
	}
	period, err := NormalizeBillingCostPeriod(BillingCostPeriod{Allocation: BillingAllocationSubscription, StartDate: "2026-10-07"}, true)
	require.NoError(t, err)
	require.Equal(t, "2026-11-07", period.EndDate)
}

func TestBillingSubscriptionMigrationKeepsLegacyCosts(t *testing.T) {
	setupBillingCostTest(t)
	require.NoError(t, DB.Migrator().DropTable(&BillingCost{}))
	legacy := struct {
		ID         int
		StartMonth string `gorm:"size:7;not null;index"`
		Recurring  bool
		CreatedBy  int
		CreatedAt  int64
		UpdatedBy  int
		UpdatedAt  int64
		Revision   int64
	}{}
	require.NoError(t, DB.Table("billing_costs").AutoMigrate(&legacy))
	legacy.ID, legacy.StartMonth, legacy.Recurring = 10, "2026-09", true
	require.NoError(t, DB.Table("billing_costs").Create(&legacy).Error)
	require.NoError(t, DB.Create(&BillingCostVersion{CostID: 10, Month: "2026-09", Name: "Legacy", AmountCents: 5000}).Error)
	require.NoError(t, DB.AutoMigrate(&BillingCost{}))
	require.NoError(t, DB.AutoMigrate(&BillingCost{})) // idempotent migration
	rows, err := GetBillingCosts("2026-10")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(5000), rows[0].AmountCents)
	require.Equal(t, BillingAllocationMonth, rows[0].Allocation)
	var count int64
	require.NoError(t, DB.Model(&BillingCostVersion{}).Where("cost_id = ?", 10).Count(&count).Error)
	require.Equal(t, int64(1), count)
}
