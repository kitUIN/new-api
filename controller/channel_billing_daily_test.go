package controller

import (
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestCalculateBalanceQueryDailyUsed(t *testing.T) {
	dailyUsed, ok := calculateBalanceQueryDailyUsed(
		&dto.BalanceQueryResult{IsValid: true, Used: 22.5},
		&dto.BalanceQueryResult{IsValid: true, Used: 25.75},
	)
	if !ok {
		t.Fatal("expected daily usage to be available")
	}
	if dailyUsed != 3.25 {
		t.Fatalf("expected daily usage 3.25, got %f", dailyUsed)
	}

	dailyUsed, ok = calculateBalanceQueryDailyUsed(
		&dto.BalanceQueryResult{IsValid: true, Used: 22.5},
		&dto.BalanceQueryResult{IsValid: true, Used: 1.25},
	)
	if !ok {
		t.Fatal("expected reset daily usage to be available")
	}
	if dailyUsed != 1.25 {
		t.Fatalf("expected reset daily usage 1.25, got %f", dailyUsed)
	}

	if _, ok = calculateBalanceQueryDailyUsed(nil, &dto.BalanceQueryResult{IsValid: true, Used: 1.25}); ok {
		t.Fatal("expected daily usage without a baseline to be unavailable")
	}
}

func TestFormatProviderBalanceDailyReportIncludesDailyUsed(t *testing.T) {
	message := formatProviderBalanceDailyReport(providerBalanceDailyReport{
		Result: &dto.BalanceQueryResult{
			IsValid:   true,
			PlanName:  "默认套餐",
			Remaining: 37.0063,
			Used:      22.9937,
			Total:     60,
			Unit:      "USD",
		},
		DailyUsed:    3.25,
		HasDailyUsed: true,
	})
	if !strings.Contains(message, "当天使用量 3.2500") {
		t.Fatalf("expected daily usage in message, got %q", message)
	}

	message = formatProviderBalanceDailyReport(providerBalanceDailyReport{
		Result: &dto.BalanceQueryResult{IsValid: true},
	})
	if !strings.Contains(message, "当天使用量 暂无基准") {
		t.Fatalf("expected missing baseline text in message, got %q", message)
	}
}

func TestSub2APIBalanceTemplateUsesTotalRecharged(t *testing.T) {
	config := buildBalanceQueryConfig(nil, dto.BalanceQuery{Template: balanceQueryTemplateSub2API})
	if config.Extractor.TotalPath != "total_recharged" {
		t.Fatalf("expected total_recharged total path, got %q", config.Extractor.TotalPath)
	}

	result := extractBalanceQueryResult([]byte(`{"is_active":true,"remaining":37.0063,"total_recharged":60,"unit":"USD"}`), config.Extractor)
	if !result.IsValid {
		t.Fatalf("expected valid result, got %q", result.InvalidMessage)
	}
	if result.Total != 60 {
		t.Fatalf("expected total 60, got %f", result.Total)
	}
	if math.Abs(result.Used-22.9937) > 0.000001 {
		t.Fatalf("expected used 22.9937, got %f", result.Used)
	}
}

func TestSubAPIBalanceTemplateAliasUsesTotalRecharged(t *testing.T) {
	config := buildBalanceQueryConfig(nil, dto.BalanceQuery{Template: "sub_api"})
	if config.Template != balanceQueryTemplateSub2API {
		t.Fatalf("expected sub_api alias to normalize to sub2api, got %q", config.Template)
	}
	if config.Extractor.TotalPath != "total_recharged" {
		t.Fatalf("expected total_recharged total path, got %q", config.Extractor.TotalPath)
	}

	result := extractBalanceQueryResult([]byte(`{"is_active":true,"remaining":37.0063,"total_recharged":60,"unit":"USD"}`), config.Extractor)
	if !result.IsValid {
		t.Fatalf("expected valid result, got %q", result.InvalidMessage)
	}
	if result.Total != 60 {
		t.Fatalf("expected total 60, got %f", result.Total)
	}
	if math.Abs(result.Used-22.9937) > 0.000001 {
		t.Fatalf("expected used 22.9937, got %f", result.Used)
	}
}
