package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannelWithExclusionsSkipsFailedChannel(t *testing.T) {
	truncateTables(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		InitChannelCache()
	})

	priority := int64(10)
	weight := uint(1)
	channels := []*Channel{
		{
			Id:       1,
			Name:     "primary",
			Key:      "sk-primary",
			Status:   common.ChannelStatusEnabled,
			Group:    "default",
			Models:   "gpt-test",
			Priority: &priority,
			Weight:   &weight,
		},
		{
			Id:       2,
			Name:     "secondary",
			Key:      "sk-secondary",
			Status:   common.ChannelStatusEnabled,
			Group:    "default",
			Models:   "gpt-test",
			Priority: &priority,
			Weight:   &weight,
		},
	}
	for _, channel := range channels {
		require.NoError(t, DB.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	InitChannelCache()

	channel, err := GetRandomSatisfiedChannelWithExclusions("default", "gpt-test", 0, []int{1})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)
}

func TestGetRandomSatisfiedChannelWithExclusionsSkipsFailedChannelWithoutCache(t *testing.T) {
	truncateTables(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})

	priority := int64(10)
	weight := uint(1)
	channels := []*Channel{
		{
			Id:       1,
			Name:     "primary",
			Key:      "sk-primary",
			Status:   common.ChannelStatusEnabled,
			Group:    "default",
			Models:   "gpt-test",
			Priority: &priority,
			Weight:   &weight,
		},
		{
			Id:       2,
			Name:     "secondary",
			Key:      "sk-secondary",
			Status:   common.ChannelStatusEnabled,
			Group:    "default",
			Models:   "gpt-test",
			Priority: &priority,
			Weight:   &weight,
		},
	}
	for _, channel := range channels {
		require.NoError(t, DB.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}

	channel, err := GetRandomSatisfiedChannelWithExclusions("default", "gpt-test", 0, []int{1})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)
}
