package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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
