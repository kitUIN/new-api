package model

import (
	"errors"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BillingCost is the immutable identity of a single or recurring expense.
type BillingCost struct {
	ID         int    `json:"id"`
	StartMonth string `json:"start_month" gorm:"size:7;not null;index"`
	Recurring  bool   `json:"recurring"`
	Allocation string `json:"allocation" gorm:"size:16;default:month"`
	StartDate  string `json:"start_date" gorm:"size:10"`
	EndDate    string `json:"end_date" gorm:"size:10"`
	CreatedBy  int    `json:"created_by"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedBy  int    `json:"updated_by"`
	UpdatedAt  int64  `json:"updated_at"`
	Revision   int64  `json:"-"`
}

// Superseded versions are soft deleted so edits retain their audit history.
type BillingCostVersion struct {
	ID          int            `json:"id"`
	CostID      int            `json:"cost_id" gorm:"index:idx_billing_version_month,priority:1;not null"`
	Month       string         `json:"month" gorm:"size:7;index:idx_billing_version_month,priority:2;not null"`
	Name        string         `json:"name" gorm:"size:128;not null"`
	AmountCents int64          `json:"amount_cents"`
	Remark      string         `json:"remark" gorm:"size:1000"`
	Disabled    bool           `json:"disabled"`
	CreatedBy   int            `json:"created_by"`
	CreatedAt   int64          `json:"created_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type BillingCostException struct {
	ID          int    `json:"id"`
	CostID      int    `json:"cost_id" gorm:"uniqueIndex:idx_billing_exception_month,priority:1;not null"`
	Month       string `json:"month" gorm:"size:7;uniqueIndex:idx_billing_exception_month,priority:2;not null"`
	Name        string `json:"name" gorm:"size:128;not null"`
	AmountCents int64  `json:"amount_cents"`
	Remark      string `json:"remark" gorm:"size:1000"`
	Disabled    bool   `json:"disabled"`
	CreatedBy   int    `json:"created_by"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedBy   int    `json:"updated_by"`
	UpdatedAt   int64  `json:"updated_at"`
}

type BillingCostRow struct {
	ID                int    `json:"id"`
	Month             string `json:"month"`
	StartMonth        string `json:"start_month"`
	Recurring         bool   `json:"recurring"`
	Exception         bool   `json:"exception"`
	Name              string `json:"name"`
	AmountCents       int64  `json:"amount_cents"`
	Remark            string `json:"remark"`
	Allocation        string `json:"allocation"`
	CycleMonth        string `json:"cycle_month"`
	PeriodStart       string `json:"period_start"`
	PeriodEnd         string `json:"period_end"`
	PeriodAmountCents int64  `json:"period_amount_cents"`
	AllocatedDays     int    `json:"allocated_days"`
	PeriodDays        int    `json:"period_days"`
}

func CreateBillingCost(month, name, remark string, cents int64, recurring bool, actor int) error {
	return CreateBillingCostWithPeriod(month, name, remark, cents, recurring, actor, BillingCostPeriod{})
}

func CreateBillingCostWithPeriod(month, name, remark string, cents int64, recurring bool, actor int, period BillingCostPeriod) error {
	period, err := NormalizeBillingCostPeriod(period, recurring)
	if err != nil {
		return err
	}
	if period.Allocation == BillingAllocationSubscription {
		month = period.StartDate[:7]
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		cost := BillingCost{StartMonth: month, Recurring: recurring, CreatedBy: actor, CreatedAt: now, UpdatedBy: actor, UpdatedAt: now}
		cost.Allocation, cost.StartDate, cost.EndDate = period.Allocation, period.StartDate, period.EndDate
		if err := tx.Create(&cost).Error; err != nil {
			return err
		}
		return tx.Create(&BillingCostVersion{CostID: cost.ID, Month: month, Name: name, Remark: remark, AmountCents: cents, CreatedBy: actor, CreatedAt: now}).Error
	})
}

// scope=month writes an exception; scope=future replaces the schedule from month.
func ChangeBillingCost(id int, month, scope, name, remark string, cents int64, disabled bool, actor int) error {
	if scope != "month" && scope != "future" {
		return errors.New("invalid cost scope")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		// Serialize writers on the parent on every supported database (including SQLite).
		locked := tx.Model(&BillingCost{}).Where("id = ?", id).Updates(map[string]interface{}{"revision": gorm.Expr("revision + 1"), "updated_by": actor, "updated_at": now})
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		var cost BillingCost
		if err := tx.First(&cost, id).Error; err != nil {
			return err
		}
		if month < cost.StartMonth || (!cost.Recurring && month != cost.StartMonth) {
			return errors.New("month is outside the cost schedule")
		}
		if !cost.Recurring && scope == "future" {
			return errors.New("single costs only support month scope")
		}
		var current BillingCostVersion
		if err := tx.Where("cost_id = ? AND month <= ?", id, month).Order("month DESC, id DESC").First(&current).Error; err != nil {
			return err
		}
		if current.Disabled {
			return errors.New("cost schedule has stopped")
		}
		if scope == "month" {
			row := BillingCostException{CostID: id, Month: month, Name: name, Remark: remark, AmountCents: cents, Disabled: disabled, CreatedBy: actor, CreatedAt: now, UpdatedBy: actor, UpdatedAt: now}
			return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "cost_id"}, {Name: "month"}}, DoUpdates: clause.AssignmentColumns([]string{"name", "remark", "amount_cents", "disabled", "updated_by", "updated_at"})}).Create(&row).Error
		}
		if disabled {
			name, remark, cents = current.Name, current.Remark, current.AmountCents
		}
		if err := tx.Where("cost_id = ? AND month >= ?", id, month).Delete(&BillingCostVersion{}).Error; err != nil {
			return err
		}
		return tx.Create(&BillingCostVersion{CostID: id, Month: month, Name: name, Remark: remark, AmountCents: cents, Disabled: disabled, CreatedBy: actor, CreatedAt: now}).Error
	})
}

func GetBillingCosts(month string) ([]BillingCostRow, error) {
	monthDate, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, err
	}
	previousMonth := monthDate.AddDate(0, -1, 0).Format("2006-01")
	rows := make([]BillingCostRow, 0)
	err = DB.Transaction(func(tx *gorm.DB) error {
		var costs []BillingCost
		if err := tx.Where("start_month <= ? AND (recurring = ? OR start_month = ? OR (allocation = ? AND end_date >= ?))", month, true, month, BillingAllocationSubscription, month+"-01").Find(&costs).Error; err != nil {
			return err
		}
		if len(costs) == 0 {
			return nil
		}
		ids := make([]int, 0, len(costs))
		exceptionMonths := []string{month, previousMonth}
		for _, cost := range costs {
			ids = append(ids, cost.ID)
			if cost.Allocation == BillingAllocationSubscription && !cost.Recurring {
				exceptionMonths = append(exceptionMonths, cost.StartMonth)
			}
		}
		var versions []BillingCostVersion
		if err := tx.Where("cost_id IN ? AND month <= ?", ids, month).Order("month ASC, id ASC").Find(&versions).Error; err != nil {
			return err
		}
		byCost := make(map[int][]BillingCostVersion)
		for _, version := range versions {
			byCost[version.CostID] = append(byCost[version.CostID], version)
		}
		var exceptions []BillingCostException
		if err := tx.Where("cost_id IN ? AND month IN ?", ids, exceptionMonths).Find(&exceptions).Error; err != nil {
			return err
		}
		overrides := make(map[int]map[string]BillingCostException)
		for _, exception := range exceptions {
			if overrides[exception.CostID] == nil {
				overrides[exception.CostID] = make(map[string]BillingCostException)
			}
			overrides[exception.CostID][exception.Month] = exception
		}
		for _, cost := range costs {
			cycles := []string{month}
			if cost.Allocation == BillingAllocationSubscription {
				cycles = []string{cost.StartMonth}
				if cost.Recurring {
					cycles = []string{previousMonth, month}
				}
			}
			for _, cycle := range cycles {
				row, err := resolveBillingCostRow(cost, cycle, month, byCost[cost.ID], overrides[cost.ID])
				if err != nil {
					return err
				}
				if row != nil {
					rows = append(rows, *row)
				}
			}
		}
		return nil
	})
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ID == rows[j].ID {
			return rows[i].CycleMonth < rows[j].CycleMonth
		}
		return rows[i].ID < rows[j].ID
	})
	return rows, err
}

func BillingPaidTopUps(start, end int64) *gorm.DB {
	return DB.Model(&TopUp{}).Where("complete_time >= ? AND complete_time < ? AND complete_time > 0 AND status IN ?", start, end, []string{common.TopUpStatusSuccess, common.TopUpStatusFrozen, common.TopUpStatusPartialRefund, common.TopUpStatusRefunded})
}

type BillingTopUpRow struct {
	ID                     int     `json:"id"`
	UserID                 int     `json:"user_id"`
	TradeNo                string  `json:"trade_no"`
	PaymentMethod          string  `json:"payment_method"`
	CompleteTime           int64   `json:"complete_time"`
	Money                  float64 `json:"money"`
	ProviderRefundedAmount int64   `json:"refunded_cents"`
	Status                 string  `json:"status"`
}

func GetBillingTopUps(start, end int64, page, size int) ([]BillingTopUpRow, int64, error) {
	rows := make([]BillingTopUpRow, 0)
	var total int64
	if err := BillingPaidTopUps(start, end).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := BillingPaidTopUps(start, end).Select("id, user_id, trade_no, payment_method, complete_time, money, provider_refunded_amount, status").Order("complete_time DESC, id DESC").Offset((page - 1) * size).Limit(size).Scan(&rows).Error
	return rows, total, err
}

type BillingGroupQuota struct {
	Group string `json:"group"`
	Quota int64  `json:"quota"`
}

func GetBillingGroupQuotas(start, end int64) ([]BillingGroupQuota, error) {
	rows := make([]BillingGroupQuota, 0)
	err := billingGroupQuotaQuery(DB, start, end).Find(&rows).Error
	return rows, err
}

func billingGroupQuotaQuery(db *gorm.DB, start, end int64) *gorm.DB {
	return db.Model(&QuotaData{}).Clauses(
		clause.Select{Expression: clause.Expr{SQL: "?, COALESCE(SUM(?), 0) AS quota", Vars: []interface{}{clause.Column{Name: "group"}, clause.Column{Name: "quota"}}}},
		clause.GroupBy{Columns: []clause.Column{{Name: "group"}}},
		clause.OrderBy{Columns: []clause.OrderByColumn{{Column: clause.Column{Name: "group"}}}},
	).Where("created_at >= ? AND created_at < ?", start, end)
}

type BillingRechargeTotals struct {
	Received      decimal.Decimal
	RefundedCents int64
}

func GetBillingRechargeTotals(start, end int64) (BillingRechargeTotals, error) {
	var result BillingRechargeTotals
	err := BillingPaidTopUps(start, end).Select("COALESCE(SUM(money), 0) AS received, COALESCE(SUM(provider_refunded_amount), 0) AS refunded_cents").Scan(&result).Error
	return result, err
}

func GetBillingBalanceQuota() (int64, error) {
	var result struct{ Quota int64 }
	err := DB.Model(&User{}).Select("COALESCE(SUM(quota), 0) AS quota").Scan(&result).Error
	return result.Quota, err
}
