package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetChannelBlacklistTestTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}))
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
		require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	})
}

func insertChannelBlacklistCandidate(t *testing.T, id int, priority int64, blacklist []int) {
	t.Helper()
	channel := &Channel{
		Id:       id,
		Type:     1,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Models:   "gpt-test",
		Group:    "default",
		Priority: &priority,
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{BlacklistUserIds: blacklist})
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
}

func TestGetChannelFallsBackAfterUserBlacklist(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	insertChannelBlacklistCandidate(t, 901, 10, []int{42})
	insertChannelBlacklistCandidate(t, 902, 5, nil)

	channel, err := GetChannel("default", "gpt-test", 0, "/v1/chat/completions", 42)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 902, channel.Id)

	channel, err = GetChannel("default", "gpt-test", 0, "/v1/chat/completions", 7)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 901, channel.Id)
}

func TestCachedChannelSelectionFallsBackAfterUserBlacklist(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		InitChannelCache()
	})
	insertChannelBlacklistCandidate(t, 903, 10, []int{42})
	insertChannelBlacklistCandidate(t, 904, 5, nil)
	InitChannelCache()

	channel, err := GetRandomSatisfiedChannel("default", "gpt-test", 0, "/v1/chat/completions", 42)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 904, channel.Id)
}

func TestGetChannelUsesNormalizedModelAfterExactCandidatesAreBlacklisted(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	exactPriority := int64(10)
	exact := &Channel{
		Id:       905,
		Type:     1,
		Key:      "key-905",
		Status:   common.ChannelStatusEnabled,
		Name:     "channel-905",
		Models:   "gpt-4-gizmo-customer",
		Group:    "default",
		Priority: &exactPriority,
	}
	exact.SetOtherSettings(dto.ChannelOtherSettings{BlacklistUserIds: []int{42}})
	require.NoError(t, DB.Create(exact).Error)
	require.NoError(t, exact.AddAbilities(nil))

	normalizedPriority := int64(5)
	normalized := &Channel{
		Id:       906,
		Type:     1,
		Key:      "key-906",
		Status:   common.ChannelStatusEnabled,
		Name:     "channel-906",
		Models:   "gpt-4-gizmo-*",
		Group:    "default",
		Priority: &normalizedPriority,
	}
	require.NoError(t, DB.Create(normalized).Error)
	require.NoError(t, normalized.AddAbilities(nil))

	channel, err := GetChannel("default", "gpt-4-gizmo-customer", 0, "/v1/chat/completions", 42)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 906, channel.Id)
}

func TestGetChannelTreatsNilPriorityAsZero(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	positivePriority := int64(5)
	positive := &Channel{
		Id:       907,
		Type:     1,
		Key:      "key-907",
		Status:   common.ChannelStatusEnabled,
		Name:     "channel-907",
		Models:   "gpt-test",
		Group:    "default",
		Priority: &positivePriority,
	}
	require.NoError(t, DB.Create(positive).Error)
	require.NoError(t, positive.AddAbilities(nil))
	require.NoError(t, DB.Create(&Channel{
		Id:     908,
		Type:   1,
		Key:    "key-908",
		Status: common.ChannelStatusEnabled,
		Name:   "channel-908",
		Models: "gpt-test",
		Group:  "default",
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "gpt-test",
		ChannelId: 908,
		Enabled:   true,
		Priority:  nil,
	}).Error)

	channel, err := GetChannel("default", "gpt-test", 0, "/v1/chat/completions", 42)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 907, channel.Id)
}
