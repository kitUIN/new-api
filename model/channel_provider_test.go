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

	updatedSource, err := GetChannelById(source.Id, true)
	require.NoError(t, err)
	sourceChannelSettings := updatedSource.GetOtherSettings()
	require.False(t, sourceChannelSettings.BalanceQuery.Enabled)
	require.False(t, sourceChannelSettings.GroupQuery.Enabled)

	updatedConsumer, err := GetChannelById(consumer.Id, true)
	require.NoError(t, err)
	consumerChannelSettings := updatedConsumer.GetOtherSettings()
	require.False(t, consumerChannelSettings.BalanceQuery.Enabled)
	require.False(t, consumerChannelSettings.GroupQuery.Enabled)
}

func TestClearChannelQuerySettingsForProviderOnlyRemovesQueryFields(t *testing.T) {
	truncateTables(t)

	provider := &ChannelProvider{
		Id:      1,
		Name:    "provider",
		BaseURL: "https://provider.example",
		Status:  1,
	}
	require.NoError(t, DB.Create(provider).Error)

	channelSettings, err := common.Marshal(dto.ChannelOtherSettings{
		AzureResponsesVersion: "2024-10-01",
		BalanceQuery: dto.BalanceQuery{
			Enabled: true,
			Request: dto.BalanceQueryRequestConfig{URL: "https://provider.example/balance"},
		},
		GroupQuery: dto.GroupQuery{
			Enabled: true,
			Request: dto.BalanceQueryRequestConfig{URL: "https://provider.example/groups"},
		},
	})
	require.NoError(t, err)
	channel := &Channel{
		Id:            10,
		ProviderID:    provider.Id,
		Name:          "channel",
		Key:           "key",
		Status:        1,
		OtherSettings: string(channelSettings),
	}
	require.NoError(t, DB.Create(channel).Error)

	require.NoError(t, ClearChannelQuerySettingsForProvider(provider.Id))

	updated, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	settings := updated.GetOtherSettings()
	require.Equal(t, "2024-10-01", settings.AzureResponsesVersion)
	require.False(t, settings.BalanceQuery.Enabled)
	require.False(t, settings.GroupQuery.Enabled)
}

func TestBuildChannelProviderTreesPlacesEnabledProvidersFirst(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&ChannelProvider{
		Id:      1,
		Name:    "enabled-provider",
		BaseURL: "https://enabled.example.com",
		Status:  common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&ChannelProvider{
		Id:      2,
		Name:    "disabled-provider",
		BaseURL: "https://disabled.example.com",
		Status:  common.ChannelStatusEnabled,
	}).Error)

	channels := []*Channel{
		{
			Id:         1,
			ProviderID: 1,
			Name:       "enabled-channel",
			Status:     common.ChannelStatusEnabled,
			Group:      "default",
		},
		{
			Id:         2,
			ProviderID: 2,
			Name:       "disabled-channel",
			Status:     common.ChannelStatusManuallyDisabled,
			Group:      "default",
		},
	}

	trees, total := BuildChannelProviderTrees(channels, 0, 20, true)
	require.Equal(t, int64(2), total)
	require.Len(t, trees, 2)
	require.Equal(t, 1, trees[0].ProviderID)
	require.Equal(t, 1, trees[0].EnabledCount)
	require.Equal(t, 2, trees[1].ProviderID)
	require.Zero(t, trees[1].EnabledCount)
}
