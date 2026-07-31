package controller

import (
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestSetChannelTestBillingRequestInputPreservesIncomingRequest(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RequestHeaders: map[string]string{"Content-Type": "application/json"},
	}
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-5.6-terra",
		Messages: []dto.Message{{
			Role:    "user",
			Content: "hi",
		}},
	}

	if err := setChannelTestBillingRequestInput(info, request); err != nil {
		t.Fatal(err)
	}
	request.SetModelName("mapped-upstream-model")

	if info.BillingRequestInput == nil {
		t.Fatal("billing request input was not set")
	}
	if got := info.BillingRequestInput.Headers["Content-Type"]; got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body struct {
		Model string `json:"model"`
	}
	if err := common.Unmarshal(info.BillingRequestInput.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Model != "gpt-5.6-terra" {
		t.Fatalf("billing model = %q, want incoming model", body.Model)
	}
}

func TestJuiceTestEligibility(t *testing.T) {
	mapping := `{"alias":"gpt-5.6-sol"}`
	tests := []struct {
		name    string
		channel *model.Channel
		want    bool
	}{
		{name: "direct model", channel: &model.Channel{Status: common.ChannelStatusEnabled, Models: "gpt-5.6-sol"}, want: true},
		{name: "mapped model", channel: &model.Channel{Status: common.ChannelStatusEnabled, Models: "alias", ModelMapping: &mapping}, want: true},
		{name: "different model", channel: &model.Channel{Status: common.ChannelStatusEnabled, Models: "gpt-5.4"}},
		{name: "disabled", channel: &model.Channel{Status: common.ChannelStatusManuallyDisabled, Models: "gpt-5.6-sol"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isJuiceTestEligible(tt.channel); got != tt.want {
				t.Fatalf("isJuiceTestEligible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildJuiceTestRequest(t *testing.T) {
	request, err := buildJuiceTestRequest()
	if err != nil {
		t.Fatal(err)
	}
	if request.Model != model.JuiceTestModel {
		t.Fatalf("model = %q, want %q", request.Model, model.JuiceTestModel)
	}
	if request.Stream == nil || *request.Stream {
		t.Fatal("juice request must explicitly disable streaming")
	}
	if request.Reasoning == nil || request.Reasoning.Effort != "max" {
		t.Fatal("juice request must use max reasoning effort")
	}
	var input string
	if err := common.Unmarshal(request.Input, &input); err != nil {
		t.Fatal(err)
	}
	if input != juiceTestPrompt {
		t.Fatalf("input = %q, want exact juice prompt", input)
	}
}

func TestExtractJuiceValue(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    string
		wantErr bool
	}{
		{name: "integer", text: "128", want: "128"},
		{name: "surrounding whitespace", text: "  256\n", want: "256"},
		{name: "decimal", text: "12.5", want: "12.5"},
		{name: "extra text", text: "Juice: 128", wantErr: true},
		{name: "empty", text: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := common.Marshal(dto.OpenAIResponsesResponse{
				Output: []dto.ResponsesOutput{{
					Type:    "message",
					Role:    "assistant",
					Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: tt.text}},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := extractJuiceValue(body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractJuiceValue() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("extractJuiceValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractJuiceValueIncludesUpstreamContentOnError(t *testing.T) {
	body := []byte(`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Juice: 128"}]}]}`)
	_, err := extractJuiceValue(body)
	if err == nil {
		t.Fatal("extractJuiceValue() should reject extra output text")
	}
	if !strings.Contains(err.Error(), "Juice: 128") || !strings.Contains(err.Error(), "upstream content") {
		t.Fatalf("error = %q, want extracted and upstream content", err)
	}
}

func TestExtractJuiceValueAcceptsObjectContent(t *testing.T) {
	body := []byte(`{
		"output": [{
			"type": "message",
			"role": "assistant",
			"content": {"type": "output_text", "text": " 512\n"}
		}]
	}`)

	juice, err := extractJuiceValue(body)
	if err != nil {
		t.Fatalf("extractJuiceValue() error = %v", err)
	}
	if juice != "512" {
		t.Fatalf("extractJuiceValue() = %q, want %q", juice, "512")
	}
}

func TestShouldRunJuiceTestUsesSixHourAttemptInterval(t *testing.T) {
	now := int64(1_000_000)
	channel := &model.Channel{Status: common.ChannelStatusEnabled, Models: model.JuiceTestModel}
	if !shouldRunJuiceTest(channel, now) {
		t.Fatal("untested eligible channel should run immediately")
	}
	channel.JuiceTestTime = now - int64(juiceTestInterval.Seconds()) + 1
	if shouldRunJuiceTest(channel, now) {
		t.Fatal("channel should not run before six hours")
	}
	channel.JuiceTestTime = now - int64(juiceTestInterval.Seconds())
	if !shouldRunJuiceTest(channel, now) {
		t.Fatal("channel should run at the six-hour boundary")
	}
}

func TestExecuteJuiceTestRejectsConcurrentChannelTest(t *testing.T) {
	channel := &model.Channel{Id: 987654, Status: common.ChannelStatusEnabled, Models: model.JuiceTestModel}
	lock := &sync.Mutex{}
	lock.Lock()
	juiceTestLocks.Store(channel.Id, lock)
	t.Cleanup(func() {
		lock.Unlock()
		juiceTestLocks.Delete(channel.Id)
	})

	_, err := executeJuiceTest(channel)
	if !errors.Is(err, errJuiceTestRunning) {
		t.Fatalf("executeJuiceTest() error = %v, want %v", err, errJuiceTestRunning)
	}
}

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
