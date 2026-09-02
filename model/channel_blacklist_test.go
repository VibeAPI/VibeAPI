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

func TestUpdateChannelBlacklistsPreservesSettingsAndRefreshesSelection(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		InitChannelCache()
	})
	insertChannelBlacklistCandidate(t, 909, 10, []int{7})
	insertChannelBlacklistCandidate(t, 910, 5, []int{8})

	var protected Channel
	require.NoError(t, DB.First(&protected, "id = ?", 909).Error)
	protectedSettings := protected.GetOtherSettings()
	protectedSettings.DisableStore = true
	protected.SetOtherSettings(protectedSettings)
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", protected.Id).Update("settings", protected.OtherSettings).Error)
	InitChannelCache()

	require.NoError(t, AddUserToChannelBlacklists([]int{909, 909}, 42))
	require.NoError(t, AddUserToChannelBlacklists([]int{909}, 42))

	var updated Channel
	require.NoError(t, DB.First(&updated, "id = ?", 909).Error)
	settings := updated.GetOtherSettings()
	assert.True(t, settings.DisableStore)
	assert.Equal(t, []int{7, 42}, settings.BlacklistUserIds)

	channel, err := GetRandomSatisfiedChannel("default", "gpt-test", 0, "/v1/chat/completions", 42)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 910, channel.Id)

	require.NoError(t, RemoveUserFromChannelBlacklists([]int{909, 910}, 42))
	require.NoError(t, DB.First(&updated, "id = ?", 909).Error)
	assert.Equal(t, []int{7}, updated.GetOtherSettings().BlacklistUserIds)

	channel, err = GetRandomSatisfiedChannel("default", "gpt-test", 0, "/v1/chat/completions", 42)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 909, channel.Id)
}

func TestAddUserToChannelBlacklistsRollsBackWhenAChannelIsMissing(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	insertChannelBlacklistCandidate(t, 911, 10, []int{7})

	err := AddUserToChannelBlacklists([]int{911, 999999}, 42)
	require.Error(t, err)

	var channel Channel
	require.NoError(t, DB.First(&channel, "id = ?", 911).Error)
	assert.Equal(t, []int{7}, channel.GetOtherSettings().BlacklistUserIds)
}

func TestListChannelBlacklistUsersReturnsUnionAndChannelCounts(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	insertChannelBlacklistCandidate(t, 912, 10, []int{41, 42})
	insertChannelBlacklistCandidate(t, 913, 5, []int{42, 43})
	require.NoError(t, DB.Create(&User{Id: 42, Username: "audited-user", DisplayName: "Audited User", Email: "audit@example.com", Remark: "manual review"}).Error)

	users, err := ListChannelBlacklistUsers([]int{912, 913})
	require.NoError(t, err)
	require.Len(t, users, 3)
	assert.Equal(t, []int{41, 42, 43}, []int{users[0].UserId, users[1].UserId, users[2].UserId})
	assert.Equal(t, 1, users[0].ChannelCount)
	assert.Equal(t, 2, users[1].ChannelCount)
	assert.Equal(t, "audited-user", users[1].Username)
	assert.Equal(t, "audit@example.com", users[1].Email)
	assert.Equal(t, "manual review", users[1].Remark)
	assert.Equal(t, 1, users[2].ChannelCount)
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
