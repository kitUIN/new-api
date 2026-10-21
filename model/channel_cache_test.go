package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// 渠道分组含空段（多余逗号/空白）或分组在 abilities 表中缺失时，
// InitChannelCache 不应因写入 nil map 而 panic。
func TestInitChannelCacheToleratesMalformedGroups(t *testing.T) {
	truncateTables(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		InitChannelCache()
	})

	priority := int64(10)
	weight := uint(1)
	newChannel := func(id int, group string) *Channel {
		return &Channel{
			Id:       id,
			Name:     "channel",
			Key:      "sk-test",
			Status:   common.ChannelStatusEnabled,
			Group:    group,
			Models:   "gpt-test",
			Priority: &priority,
			Weight:   &weight,
		}
	}

	// 含空分组段：AddAbilities 会跳过空段，abilities 中没有 "" / " " 分组
	trailingComma := newChannel(1, "default,, ,")
	// 分组整体为空
	emptyGroup := newChannel(2, "")
	// 分组未写入 abilities（例如写入失败或数据被直接修改）
	missingAbility := newChannel(3, "orphan")

	for _, channel := range []*Channel{trailingComma, emptyGroup, missingAbility} {
		require.NoError(t, DB.Create(channel).Error)
	}
	require.NoError(t, trailingComma.AddAbilities(nil))
	require.NoError(t, emptyGroup.AddAbilities(nil))

	require.NotPanics(t, InitChannelCache)

	channelSyncLock.RLock()
	_, hasEmpty := group2model2channels[""]
	_, hasBlank := group2model2channels[" "]
	channelSyncLock.RUnlock()
	require.False(t, hasEmpty)
	require.False(t, hasBlank)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-test", 0)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, trailingComma.Id, channel.Id)

	channel, err = GetRandomSatisfiedChannel("orphan", "gpt-test", 0)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, missingAbility.Id, channel.Id)
}
