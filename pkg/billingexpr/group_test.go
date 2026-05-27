package billingexpr_test

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
)

func TestRunExpr_GroupCondition(t *testing.T) {
	exprStr := `group == "vip" ? tier("vip", p * 1) : tier("base", p * 2)`

	cost, trace, err := billingexpr.RunExprWithRequest(
		exprStr,
		billingexpr.TokenParams{P: 1000},
		billingexpr.RequestInput{Group: "vip"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if cost != 1000 {
		t.Fatalf("cost = %v, want 1000", cost)
	}
	if trace.MatchedTier != "vip" {
		t.Fatalf("matched tier = %q, want vip", trace.MatchedTier)
	}
}

func TestComputeTieredQuota_UsesSnapshotGroupFallback(t *testing.T) {
	exprStr := `group == "vip" ? tier("vip", p * 1) : tier("base", p * 2)`
	snap := &billingexpr.BillingSnapshot{
		BillingMode:  "tiered_expr",
		ExprString:   exprStr,
		ExprHash:     billingexpr.ExprHashString(exprStr),
		GroupRatio:   1,
		QuotaPerUnit: 500000,
		Group:        "vip",
	}

	result, err := billingexpr.ComputeTieredQuota(snap, billingexpr.TokenParams{P: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchedTier != "vip" {
		t.Fatalf("matched tier = %q, want vip", result.MatchedTier)
	}
	if result.ActualQuotaAfterGroup != 500 {
		t.Fatalf("quota = %d, want 500", result.ActualQuotaAfterGroup)
	}
}
