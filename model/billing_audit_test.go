package model

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupBillingCostTest(t *testing.T) {
	t.Helper()
	previous := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	t.Cleanup(func() { DB = previous; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&BillingCost{}, &BillingCostVersion{}, &BillingCostException{}))
}

func TestBillingCostRecurrenceHistoryAndOverrides(t *testing.T) {
	setupBillingCostTest(t)
	require.NoError(t, CreateBillingCost("2026-12", "Server", "", 3000, true, 1))
	rows, err := GetBillingCosts("2026-11")
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = GetBillingCosts("2026-12")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	id := rows[0].ID
	require.NoError(t, ChangeBillingCost(id, "2027-02", "future", "Server v2", "upgrade", 4000, false, 2))
	require.NoError(t, ChangeBillingCost(id, "2027-03", "month", "Discount", "", 1000, false, 2))
	require.NoError(t, ChangeBillingCost(id, "2027-04", "month", "", "", 0, true, 2))
	for _, test := range []struct {
		month string
		cents int64
	}{{"2026-12", 3000}, {"2027-01", 3000}, {"2027-02", 4000}, {"2027-03", 1000}, {"2027-04", 0}, {"2027-05", 4000}, {"2030-01", 4000}} {
		rows, err := GetBillingCosts(test.month)
		require.NoError(t, err)
		if test.cents == 0 {
			require.Empty(t, rows)
			continue
		}
		require.Len(t, rows, 1)
		require.Equal(t, test.cents, rows[0].AmountCents, test.month)
	}
	// Stopping overrides even pre-existing exceptions in future months.
	require.NoError(t, ChangeBillingCost(id, "2027-06", "month", "Future override", "", 9000, false, 2))
	require.NoError(t, ChangeBillingCost(id, "2027-05", "future", "", "", 0, true, 3))
	for _, month := range []string{"2027-05", "2027-06", "2028-01"} {
		rows, err := GetBillingCosts(month)
		require.NoError(t, err)
		require.Empty(t, rows)
	}
	rows, err = GetBillingCosts("2027-03")
	require.NoError(t, err)
	require.Equal(t, int64(1000), rows[0].AmountCents)
	require.Error(t, ChangeBillingCost(id, "2027-06", "month", "No restart", "", 100, false, 2))
	var count int64
	require.NoError(t, DB.Model(&BillingCost{}).Count(&count).Error)
	require.Equal(t, int64(1), count) // Querying never materializes duplicate monthly rows.
}

func TestBillingCostVersionsSingleCostAndValidation(t *testing.T) {
	setupBillingCostTest(t)
	require.NoError(t, CreateBillingCost("2026-09", "Single", "", 2000, false, 1))
	rows, err := GetBillingCosts("2026-09")
	require.NoError(t, err)
	id := rows[0].ID
	require.Error(t, ChangeBillingCost(id, "2026-08", "month", "x", "", 100, false, 2))
	require.Error(t, ChangeBillingCost(id, "2026-10", "month", "x", "", 100, false, 2))
	require.Error(t, ChangeBillingCost(id, "2026-09", "future", "x", "", 100, false, 2))
	require.Error(t, ChangeBillingCost(id, "2026-09", "all", "x", "", 100, false, 2))
	require.Error(t, ChangeBillingCost(999, "2026-09", "month", "x", "", 100, false, 2))
	require.NoError(t, ChangeBillingCost(id, "2026-09", "month", "Adjusted", "", 2500, false, 2))
	require.NoError(t, ChangeBillingCost(id, "2026-09", "month", "Adjusted again", "", 2600, false, 3))
	var overrides []BillingCostException
	require.NoError(t, DB.Find(&overrides).Error)
	require.Len(t, overrides, 1)
	require.Equal(t, 2, overrides[0].CreatedBy)
	require.Equal(t, 3, overrides[0].UpdatedBy)
	require.NoError(t, ChangeBillingCost(id, "2026-09", "month", "", "", 0, true, 3))
	rows, err = GetBillingCosts("2026-09")
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = GetBillingCosts("2026-10")
	require.NoError(t, err)
	require.Empty(t, rows)
	// Replacing the future schedule preserves the superseded version for auditing.
	require.NoError(t, CreateBillingCost("2026-09", "Recurring", "", 3000, true, 1))
	rows, err = GetBillingCosts("2026-09")
	require.NoError(t, err)
	id = rows[0].ID
	require.NoError(t, ChangeBillingCost(id, "2026-11", "future", "v2", "", 4000, false, 2))
	require.NoError(t, ChangeBillingCost(id, "2026-10", "future", "v3", "", 5000, false, 3))
	rows, err = GetBillingCosts("2026-11")
	require.NoError(t, err)
	require.Equal(t, int64(5000), rows[0].AmountCents)
	var history []BillingCostVersion
	require.NoError(t, DB.Unscoped().Where("cost_id = ?", id).Find(&history).Error)
	require.Len(t, history, 3)
	for _, version := range history {
		if version.Month == "2026-11" {
			require.True(t, version.DeletedAt.Valid)
		}
	}
}

func TestBillingAuditDialectQueriesAndSchemas(t *testing.T) {
	for name, dialect := range map[string]gorm.Dialector{
		"sqlite":   sqlite.Open(":memory:"),
		"mysql":    mysql.New(mysql.Config{SkipInitializeWithVersion: true}),
		"postgres": postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}),
	} {
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(dialect, &gorm.Config{DisableAutomaticPing: true, DryRun: true, SkipDefaultTransaction: true})
			require.NoError(t, err)
			var rows []BillingGroupQuota
			query := billingGroupQuotaQuery(db, 100, 200).Find(&rows)
			require.NoError(t, query.Error)
			quoted := "`group`"
			if name == "postgres" {
				quoted = `"group"`
			}
			require.Contains(t, query.Statement.SQL.String(), "GROUP BY "+quoted)
			require.Contains(t, query.Statement.SQL.String(), "COALESCE(SUM(")
			for _, entity := range []interface{}{&BillingCost{}, &BillingCostVersion{}, &BillingCostException{}} {
				statement := &gorm.Statement{DB: db}
				require.NoError(t, statement.Parse(entity))
				for _, field := range statement.Schema.Fields {
					dataType := strings.ToUpper(db.Migrator().FullDataTypeOf(field).SQL)
					require.NotContains(t, dataType, "JSONB")
					if field.Name == "AmountCents" {
						require.Contains(t, dataType, "INT")
					}
				}
			}
		})
	}
}
