package controller

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestSub2APIGroupTemplateDefaults(t *testing.T) {
	config := buildGroupQueryConfig(nil, dto.GroupQuery{Template: "sub_api"})

	require.Equal(t, balanceQueryTemplateSub2API, config.Template)
	require.Equal(t, "{{baseUrl}}/api/v1/groups/available?timezone=Asia%2FShanghai", config.Request.URL)
	require.Equal(t, "GET", config.Request.Method)
	require.Equal(t, "Bearer {{accessToken}}", config.Request.Headers["Authorization"])
	require.Equal(t, "data", config.Extractor.DataPath)
	require.Equal(t, "description", config.Extractor.DescPath)
	require.Equal(t, "rate_multiplier", config.Extractor.RatioPath)
	require.Equal(t, "code", config.Extractor.SuccessPath)
	require.Equal(t, "0", config.Extractor.SuccessValue)
	require.False(t, config.Extractor.SuccessOptional)
}

func TestExtractGroupQueryResultSupportsSub2APIArray(t *testing.T) {
	config := buildGroupQueryConfig(nil, dto.GroupQuery{Template: balanceQueryTemplateSub2API})
	body := []byte(`{
		"code": 0,
		"message": "success",
		"data": [
			{"id": 2, "name": "GPT-VIP专线", "description": "", "rate_multiplier": 0.15},
			{"id": 13, "name": "GPT-Pro通道", "description": "Pro", "rate_multiplier": 0.2}
		]
	}`)

	result, err := extractGroupQueryResult(body, config.Extractor)

	require.NoError(t, err)
	require.Equal(t, dto.GroupQueryItem{Desc: "GPT-VIP专线", Ratio: 0.15}, result["GPT-VIP专线"])
	require.Equal(t, dto.GroupQueryItem{Desc: "Pro", Ratio: 0.2}, result["GPT-Pro通道"])
}

func TestApplyUpstreamGroupRatioBindingsToMap(t *testing.T) {
	result := map[string]dto.GroupQueryItem{
		"up-default": {Desc: "Default", Ratio: 1.2},
		"up-vip":     {Desc: "VIP", Ratio: 0.8},
	}
	current := map[string]float64{
		"default": 1,
		"vip":     1,
		"locked":  0.75,
		"expr":    1,
	}
	bindings := map[string]ratio_setting.UpstreamGroupRatioBinding{
		"default": {
			SourceType:    ratio_setting.UpstreamGroupRatioBindingSourceChannel,
			SourceID:      10,
			UpstreamGroup: "up-default",
			Offset:        0.01,
		},
		"vip": {
			SourceType:    ratio_setting.UpstreamGroupRatioBindingSourceProvider,
			SourceID:      20,
			UpstreamGroup: "up-vip",
			Offset:        -0.05,
		},
		"locked": {
			SourceType:    ratio_setting.UpstreamGroupRatioBindingSourceChannel,
			SourceID:      10,
			UpstreamGroup: "up-vip",
			Offset:        -2,
		},
		"expr": {
			SourceType:       ratio_setting.UpstreamGroupRatioBindingSourceChannel,
			SourceID:         10,
			UpstreamGroup:    "up-default",
			OffsetExpression: "(x + 0.3) / 10 + 0.4",
		},
		"missing-local": {
			SourceType:    ratio_setting.UpstreamGroupRatioBindingSourceChannel,
			SourceID:      10,
			UpstreamGroup: "up-default",
		},
	}

	next, changed := applyUpstreamGroupRatioBindingsToMap(
		ratio_setting.UpstreamGroupRatioBindingSourceChannel,
		10,
		result,
		bindings,
		current,
	)

	require.True(t, changed)
	require.Equal(t, 1.21, next["default"])
	require.Equal(t, 1.0, next["vip"])
	require.Equal(t, 0.0, next["locked"])
	require.InDelta(t, 0.55, next["expr"], 1e-9)
	require.NotContains(t, next, "missing-local")
}

func TestApplyUpstreamGroupRatioBindingsToMapProvider(t *testing.T) {
	next, changed := applyUpstreamGroupRatioBindingsToMap(
		ratio_setting.UpstreamGroupRatioBindingSourceProvider,
		20,
		map[string]dto.GroupQueryItem{"up-vip": {Ratio: 0.8}},
		map[string]ratio_setting.UpstreamGroupRatioBinding{
			"vip": {
				SourceType:    ratio_setting.UpstreamGroupRatioBindingSourceProvider,
				SourceID:      20,
				UpstreamGroup: "up-vip",
				Offset:        -0.05,
			},
		},
		map[string]float64{"vip": 1},
	)

	require.True(t, changed)
	require.Equal(t, 0.75, next["vip"])
}

func TestApplyUpstreamGroupRatioBindingsToMapSkipsNoopsAndInvalid(t *testing.T) {
	tests := []struct {
		name     string
		result   map[string]dto.GroupQueryItem
		binding  ratio_setting.UpstreamGroupRatioBinding
		current  map[string]float64
		expected map[string]float64
		changed  bool
	}{
		{
			name:    "same ratio",
			result:  map[string]dto.GroupQueryItem{"up": {Ratio: 1}},
			binding: ratio_setting.UpstreamGroupRatioBinding{SourceType: "channel", SourceID: 1, UpstreamGroup: "up"},
			current: map[string]float64{"default": 1},
			changed: false,
		},
		{
			name:    "source mismatch",
			result:  map[string]dto.GroupQueryItem{"up": {Ratio: 2}},
			binding: ratio_setting.UpstreamGroupRatioBinding{SourceType: "channel", SourceID: 2, UpstreamGroup: "up"},
			current: map[string]float64{"default": 1},
			changed: false,
		},
		{
			name:    "upstream group missing",
			result:  map[string]dto.GroupQueryItem{"other": {Ratio: 2}},
			binding: ratio_setting.UpstreamGroupRatioBinding{SourceType: "channel", SourceID: 1, UpstreamGroup: "up"},
			current: map[string]float64{"default": 1},
			changed: false,
		},
		{
			name:    "invalid ratio",
			result:  map[string]dto.GroupQueryItem{"up": {Ratio: math.Inf(1)}},
			binding: ratio_setting.UpstreamGroupRatioBinding{SourceType: "channel", SourceID: 1, UpstreamGroup: "up"},
			current: map[string]float64{"default": 1},
			changed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, changed := applyUpstreamGroupRatioBindingsToMap(
				ratio_setting.UpstreamGroupRatioBindingSourceChannel,
				1,
				tt.result,
				map[string]ratio_setting.UpstreamGroupRatioBinding{"default": tt.binding},
				tt.current,
			)

			require.Equal(t, tt.changed, changed)
			require.Equal(t, tt.current, next)
		})
	}
}

func TestExtractSub2APIKeyGroup(t *testing.T) {
	const channelKey = "sk-aaa"

	tests := []struct {
		name      string
		body      string
		keys      []string
		wantName  string
		wantRatio float64
		wantNil   bool
		wantErr   bool
	}{
		{
			name: "single key single group",
			body: `{"code":0,"message":"success","data":{"items":[
				{"id":1,"key":"sk-aaa","name":"fl","group":{"id":126,"name":"Codex","rate_multiplier":0.03}},
				{"id":2,"key":"sk-bbb","name":"other","group":{"id":99,"name":"Other","rate_multiplier":1}}
			],"total":2}}`,
			keys:      []string{channelKey},
			wantName:  "Codex",
			wantRatio: 0.03,
		},
		{
			name: "multi key same group",
			body: `{"code":0,"message":"success","data":{"items":[
				{"id":1,"key":"sk-aaa","group":{"id":126,"name":"Codex","rate_multiplier":0.03}},
				{"id":2,"key":"sk-bbb","group":{"id":126,"name":"Codex","rate_multiplier":0.03}}
			],"total":2}}`,
			keys:      []string{"sk-aaa", "sk-bbb"},
			wantName:  "Codex",
			wantRatio: 0.03,
		},
		{
			name: "multi key different groups is ambiguous",
			body: `{"code":0,"message":"success","data":{"items":[
				{"id":1,"key":"sk-aaa","group":{"id":126,"name":"Codex","rate_multiplier":0.03}},
				{"id":2,"key":"sk-bbb","group":{"id":99,"name":"Other","rate_multiplier":1}}
			],"total":2}}`,
			keys:    []string{"sk-aaa", "sk-bbb"},
			wantNil: true,
		},
		{
			name:    "key not found",
			body:    `{"code":0,"message":"success","data":{"items":[{"id":2,"key":"sk-bbb","group":{"id":99,"name":"Other","rate_multiplier":1}}],"total":1}}`,
			keys:    []string{channelKey},
			wantNil: true,
		},
		{
			name:    "null group",
			body:    `{"code":0,"message":"success","data":{"items":[{"id":1,"key":"sk-aaa","group":null}],"total":1}}`,
			keys:    []string{channelKey},
			wantNil: true,
		},
		{
			name:    "upstream error code",
			body:    `{"code":401,"message":"unauthorized","data":{"items":[]}}`,
			keys:    []string{channelKey},
			wantErr: true,
		},
		{
			name:    "malformed body",
			body:    `not-json`,
			keys:    []string{channelKey},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSub2APIKeyGroup([]byte(tt.body), tt.keys)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.wantName, got.Name)
			require.Equal(t, tt.wantRatio, got.RateMultiplier)
		})
	}
}

func TestResolveUpstreamGroupBindingUpdate(t *testing.T) {
	t.Run("no binding creates one", func(t *testing.T) {
		got, changed := resolveUpstreamGroupBindingUpdate(ratio_setting.UpstreamGroupRatioBinding{}, 7, "Codex")

		require.True(t, changed)
		require.Equal(t, ratio_setting.UpstreamGroupRatioBindingSourceChannel, got.SourceType)
		require.Equal(t, 7, got.SourceID)
		require.Equal(t, "Codex", got.UpstreamGroup)
	})

	t.Run("identical binding is unchanged", func(t *testing.T) {
		existing := ratio_setting.UpstreamGroupRatioBinding{
			SourceType:    ratio_setting.UpstreamGroupRatioBindingSourceChannel,
			SourceID:      7,
			UpstreamGroup: "Codex",
			Offset:        0.01,
		}

		got, changed := resolveUpstreamGroupBindingUpdate(existing, 7, "Codex")

		require.False(t, changed)
		require.Equal(t, existing, got)
	})

	t.Run("upstream group change preserves numeric offset", func(t *testing.T) {
		existing := ratio_setting.UpstreamGroupRatioBinding{
			SourceType:    ratio_setting.UpstreamGroupRatioBindingSourceChannel,
			SourceID:      7,
			UpstreamGroup: "OldGroup",
			Offset:        0.02,
		}

		got, changed := resolveUpstreamGroupBindingUpdate(existing, 7, "Codex")

		require.True(t, changed)
		require.Equal(t, "Codex", got.UpstreamGroup)
		require.Equal(t, 0.02, got.Offset)
	})

	t.Run("upstream group change preserves offset expression", func(t *testing.T) {
		existing := ratio_setting.UpstreamGroupRatioBinding{
			SourceType:       ratio_setting.UpstreamGroupRatioBindingSourceChannel,
			SourceID:         7,
			UpstreamGroup:    "OldGroup",
			OffsetExpression: "ratio * 1.1",
		}

		got, changed := resolveUpstreamGroupBindingUpdate(existing, 7, "Codex")

		require.True(t, changed)
		require.Equal(t, "Codex", got.UpstreamGroup)
		require.Equal(t, "ratio * 1.1", got.OffsetExpression)
	})

	t.Run("rebinding from provider source takes over", func(t *testing.T) {
		existing := ratio_setting.UpstreamGroupRatioBinding{
			SourceType:    ratio_setting.UpstreamGroupRatioBindingSourceProvider,
			SourceID:      7,
			UpstreamGroup: "Codex",
		}

		got, changed := resolveUpstreamGroupBindingUpdate(existing, 7, "Codex")

		require.True(t, changed)
		require.Equal(t, ratio_setting.UpstreamGroupRatioBindingSourceChannel, got.SourceType)
	})
}
