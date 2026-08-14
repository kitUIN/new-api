package common

import (
	"net/http/httptest"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestServiceTierFromRequestRecognizesOpenAIFastAliases(t *testing.T) {
	tests := []struct {
		name    string
		request dto.Request
		want    string
	}{
		{
			name: "chat fast",
			request: &dto.GeneralOpenAIRequest{
				ServiceTier: []byte(`"fast"`),
			},
			want: "fast",
		},
		{
			name: "chat priority alias",
			request: &dto.GeneralOpenAIRequest{
				ServiceTier: []byte(`"priority"`),
			},
			want: "priority",
		},
		{
			name: "responses fast",
			request: &dto.OpenAIResponsesRequest{
				ServiceTier: "fast",
			},
			want: "fast",
		},
		{
			name: "invalid chat tier",
			request: &dto.GeneralOpenAIRequest{
				ServiceTier: []byte(`true`),
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, serviceTierFromRequest(tt.request))
		})
	}

	require.True(t, IsOpenAIFastServiceTier("fast"))
	require.True(t, IsOpenAIFastServiceTier("priority"))
	require.False(t, IsOpenAIFastServiceTier("default"))
}

func TestRefreshOpenAIFastModeBillingRequiresOpenAIPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	globalSettings := model_setting.GetGlobalSettings()
	originalGlobalPassthrough := globalSettings.PassThroughRequestEnabled
	t.Cleanup(func() {
		globalSettings.PassThroughRequestEnabled = originalGlobalPassthrough
	})

	tests := []struct {
		name                   string
		channelType            int
		tier                   string
		globalPassthrough      bool
		channelBodyPassthrough bool
		allowServiceTier       bool
		wantPassthrough        bool
		wantMultiplier         float64
	}{
		{
			name:           "filtered fast request",
			channelType:    constant.ChannelTypeOpenAI,
			tier:           "fast",
			wantMultiplier: 1,
		},
		{
			name:             "explicit service tier allowance",
			channelType:      constant.ChannelTypeOpenAI,
			tier:             "fast",
			allowServiceTier: true,
			wantPassthrough:  true,
			wantMultiplier:   OpenAIFastModeBillingMultiplier,
		},
		{
			name:                   "channel body passthrough",
			channelType:            constant.ChannelTypeOpenAI,
			tier:                   "priority",
			channelBodyPassthrough: true,
			wantPassthrough:        true,
			wantMultiplier:         OpenAIFastModeBillingMultiplier,
		},
		{
			name:              "global passthrough",
			channelType:       constant.ChannelTypeOpenAI,
			tier:              "fast",
			globalPassthrough: true,
			wantPassthrough:   true,
			wantMultiplier:    OpenAIFastModeBillingMultiplier,
		},
		{
			name:             "non OpenAI channel",
			channelType:      constant.ChannelTypeAzure,
			tier:             "fast",
			allowServiceTier: true,
			wantPassthrough:  true,
			wantMultiplier:   1,
		},
		{
			name:             "non fast tier",
			channelType:      constant.ChannelTypeOpenAI,
			tier:             "default",
			allowServiceTier: true,
			wantPassthrough:  true,
			wantMultiplier:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalSettings.PassThroughRequestEnabled = tt.globalPassthrough

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			rootcommon.SetContextKey(ctx, constant.ContextKeyChannelType, tt.channelType)
			rootcommon.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{
				PassThroughBodyEnabled: tt.channelBodyPassthrough,
			})
			rootcommon.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
				AllowServiceTier: tt.allowServiceTier,
			})

			info := &RelayInfo{
				ServiceTier:                  tt.tier,
				ServiceTierBillingMultiplier: 99,
			}
			info.RefreshOpenAIFastModeBilling(ctx)

			require.Equal(t, tt.wantPassthrough, info.ServiceTierPassthroughEnabled)
			require.Equal(t, tt.wantMultiplier, info.GetServiceTierBillingMultiplier())
		})
	}
}

func TestShouldStripServiceTierFromBillingInput(t *testing.T) {
	require.True(t, (&RelayInfo{
		ServiceTier:                   "fast",
		ServiceTierPassthroughEnabled: false,
	}).ShouldStripServiceTierFromBillingInput())
	require.True(t, (&RelayInfo{
		ServiceTier:                   "fast",
		ServiceTierPassthroughEnabled: true,
		ServiceTierBillingMultiplier:  OpenAIFastModeBillingMultiplier,
	}).ShouldStripServiceTierFromBillingInput())
	require.False(t, (&RelayInfo{
		ServiceTier:                   "default",
		ServiceTierPassthroughEnabled: true,
		ServiceTierBillingMultiplier:  1,
	}).ShouldStripServiceTierFromBillingInput())
}
