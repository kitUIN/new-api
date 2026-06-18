package controller

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestParseChannelTestStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "missing defaults to stream", url: "/api/channel/test/1", want: true},
		{name: "empty defaults to stream", url: "/api/channel/test/1?stream=", want: true},
		{name: "invalid defaults to stream", url: "/api/channel/test/1?stream=nope", want: true},
		{name: "explicit false disables stream", url: "/api/channel/test/1?stream=false", want: false},
		{name: "explicit true enables stream", url: "/api/channel/test/1?stream=true", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", tt.url, nil)

			if got := parseChannelTestStream(c); got != tt.want {
				t.Fatalf("parseChannelTestStream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeChannelTestEndpointOpenAIUsesResponses(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI}

	tests := []struct {
		name         string
		modelName    string
		endpointType string
		want         string
	}{
		{
			name:      "auto openai defaults to responses",
			modelName: "gpt-4o-mini",
			want:      string(constant.EndpointTypeOpenAIResponse),
		},
		{
			name:         "explicit openai is upgraded to responses",
			modelName:    "gpt-4o-mini",
			endpointType: string(constant.EndpointTypeOpenAI),
			want:         string(constant.EndpointTypeOpenAIResponse),
		},
		{
			name:         "specialized endpoint is preserved",
			modelName:    "text-embedding-3-small",
			endpointType: string(constant.EndpointTypeEmbeddings),
			want:         string(constant.EndpointTypeEmbeddings),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeChannelTestEndpoint(channel, tt.modelName, tt.endpointType)
			if got != tt.want {
				t.Fatalf("normalizeChannelTestEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeChannelTestModelNameUsesMappedTarget(t *testing.T) {
	modelMapping := `{"gpt-4o":"vendor-gpt-4o"}`
	channel := &model.Channel{
		Models:       "gpt-4o,claude-3-5-sonnet",
		ModelMapping: &modelMapping,
	}

	got := normalizeChannelTestModelName(channel, "vendor-gpt-4o")
	if got != "gpt-4o" {
		t.Fatalf("normalizeChannelTestModelName() = %q, want %q", got, "gpt-4o")
	}
}

func TestNormalizeChannelTestModelNamePreservesConfiguredModel(t *testing.T) {
	modelMapping := `{"gpt-4o":"vendor-gpt-4o"}`
	channel := &model.Channel{
		Models:       "gpt-4o,vendor-gpt-4o",
		ModelMapping: &modelMapping,
	}

	got := normalizeChannelTestModelName(channel, "vendor-gpt-4o")
	if got != "vendor-gpt-4o" {
		t.Fatalf("normalizeChannelTestModelName() = %q, want %q", got, "vendor-gpt-4o")
	}
}

func TestNormalizeChannelTestModelNameUsesChainedMappedTarget(t *testing.T) {
	modelMapping := `{"gpt-4o":"vendor-alias","vendor-alias":"vendor-final"}`
	channel := &model.Channel{
		Models:       "gpt-4o",
		ModelMapping: &modelMapping,
	}

	got := normalizeChannelTestModelName(channel, "vendor-final")
	if got != "gpt-4o" {
		t.Fatalf("normalizeChannelTestModelName() = %q, want %q", got, "gpt-4o")
	}
}

func TestNormalizeChannelTestModelNamePreservesCompactSuffixForMappedTarget(t *testing.T) {
	modelMapping := `{"gpt-4o":"vendor-gpt-4o"}`
	channel := &model.Channel{
		Models:       "gpt-4o",
		ModelMapping: &modelMapping,
	}

	got := normalizeChannelTestModelName(channel, "vendor-gpt-4o-openai-compact")
	want := "gpt-4o-openai-compact"
	if got != want {
		t.Fatalf("normalizeChannelTestModelName() = %q, want %q", got, want)
	}
}

func TestNormalizeChannelTestModelNameIgnoresInvalidMapping(t *testing.T) {
	modelMapping := `{`
	channel := &model.Channel{
		Models:       "gpt-4o",
		ModelMapping: &modelMapping,
	}

	got := normalizeChannelTestModelName(channel, "vendor-gpt-4o")
	if got != "vendor-gpt-4o" {
		t.Fatalf("normalizeChannelTestModelName() = %q, want %q", got, "vendor-gpt-4o")
	}
}

func TestNextAlignedChannelTestTimeUsesWallClockBuckets(t *testing.T) {
	now := time.Date(2026, 6, 12, 1, 23, 45, 0, time.Local)
	got := nextAlignedChannelTestTime(now)
	want := time.Date(2026, 6, 12, 1, 30, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("nextAlignedChannelTestTime() = %v, want %v", got, want)
	}
}

func TestNextAlignedChannelTestTimeSkipsCurrentBucketBoundary(t *testing.T) {
	now := time.Date(2026, 6, 12, 1, 30, 0, 0, time.Local)
	got := nextAlignedChannelTestTime(now)
	want := time.Date(2026, 6, 12, 1, 40, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("nextAlignedChannelTestTime() = %v, want %v", got, want)
	}
}

func TestCalculateChannelTestQuotaAppliesGroupRatio(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
	}
	priceData := types.PriceData{
		ModelRatio:      2,
		CompletionRatio: 3,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 0.5,
		},
	}

	got := calculateChannelTestQuota(usage, priceData)
	if got != 250 {
		t.Fatalf("calculateChannelTestQuota() = %d, want 250", got)
	}
}

func TestCalculateChannelTestQuotaAppliesGroupRatioForFixedPrice(t *testing.T) {
	priceData := types.PriceData{
		UsePrice:   true,
		ModelPrice: 0.01,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 2,
		},
	}

	got := calculateChannelTestQuota(&dto.Usage{TotalTokens: 1}, priceData)
	want := int(0.01 * common.QuotaPerUnit * 2)
	if got != want {
		t.Fatalf("calculateChannelTestQuota() = %d, want %d", got, want)
	}
}

func TestCalculateChannelTestQuotaZeroGroupRatioIsFree(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     1,
		CompletionTokens: 0,
	}
	priceData := types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 1,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 0,
		},
	}

	got := calculateChannelTestQuota(usage, priceData)
	if got != 0 {
		t.Fatalf("calculateChannelTestQuota() = %d, want 0", got)
	}
}
