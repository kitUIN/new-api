package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestMigrateChannelProviderQuerySettingsCompletesMissingGroupFromSharedSource(t *testing.T) {
	truncateTables(t)

	provider := &ChannelProvider{
		Id:      1,
		Name:    "provider",
		BaseURL: "https://provider.example",
		Status:  1,
	}
	provider.SetOtherSettings(dto.ChannelProviderSettings{
		BalanceQuery: dto.BalanceQuery{
			Enabled: true,
			Request: dto.BalanceQueryRequestConfig{URL: "https://provider.example/balance"},
		},
	})
	require.NoError(t, DB.Create(provider).Error)

	sourceSettings, err := common.Marshal(dto.ChannelOtherSettings{
		GroupQuery: dto.GroupQuery{
			Enabled: true,
			Request: dto.BalanceQueryRequestConfig{URL: "https://provider.example/groups-source"},
		},
	})
	require.NoError(t, err)
	source := &Channel{
		Id:            20,
		ProviderID:    provider.Id,
		Name:          "source",
		Key:           "source-key",
		Status:        1,
		OtherSettings: string(sourceSettings),
	}
	require.NoError(t, DB.Create(source).Error)

	consumerSettings, err := common.Marshal(dto.ChannelOtherSettings{
		GroupQuery: dto.GroupQuery{
			Enabled:         true,
			SourceChannelID: source.Id,
			Request:         dto.BalanceQueryRequestConfig{URL: "https://provider.example/groups-consumer"},
		},
	})
	require.NoError(t, err)
	consumer := &Channel{
		Id:            10,
		ProviderID:    provider.Id,
		Name:          "consumer",
		Key:           "consumer-key",
		Status:        1,
		OtherSettings: string(consumerSettings),
	}
	require.NoError(t, DB.Create(consumer).Error)

	require.NoError(t, MigrateChannelProviderQuerySettings())

	updated, err := GetChannelProviderByID(provider.Id)
	require.NoError(t, err)
	settings := updated.GetOtherSettings()
	require.True(t, settings.BalanceQuery.Enabled)
	require.True(t, settings.GroupQuery.Enabled)
	require.Equal(t, source.Id, settings.GroupQuery.SourceChannelID)
	require.Equal(t, "https://provider.example/groups-source", settings.GroupQuery.Request.URL)
}
