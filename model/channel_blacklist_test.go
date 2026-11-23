package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
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
	channel.SetOtherSettings(kitdto.ChannelOtherSettings{BlacklistUserIds: blacklist})
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
}

func TestGetChannelFallsBackAfterUserBlacklist(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	insertChannelBlacklistCandidate(t, 901, 10, []int{42})
	insertChannelBlacklistCandidate(t, 902, 5, nil)

	filters := []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"}, {Kind: dto.FilterUserBlacklist, UserId: 42}}
	channel, err := GetChannel("default", "gpt-test", 0, filters)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 902, channel.Id)

	filters[1].UserId = 7
	channel, err = GetChannel("default", "gpt-test", 0, filters)
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

	channel, err := GetRandomSatisfiedChannel("default", "gpt-test", 0, []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"}, {Kind: dto.FilterUserBlacklist, UserId: 42}})
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

	channel, err := GetRandomSatisfiedChannel("default", "gpt-test", 0, []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"}, {Kind: dto.FilterUserBlacklist, UserId: 42}})
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 910, channel.Id)

	require.NoError(t, RemoveUserFromChannelBlacklists([]int{909, 910}, 42))
	require.NoError(t, DB.First(&updated, "id = ?", 909).Error)
	assert.Equal(t, []int{7}, updated.GetOtherSettings().BlacklistUserIds)

	channel, err = GetRandomSatisfiedChannel("default", "gpt-test", 0, []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"}, {Kind: dto.FilterUserBlacklist, UserId: 42}})
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
	exact.SetOtherSettings(kitdto.ChannelOtherSettings{BlacklistUserIds: []int{42}})
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

	channel, err := GetChannel("default", "gpt-4-gizmo-customer", 0, []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"}, {Kind: dto.FilterUserBlacklist, UserId: 42}})
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

	channel, err := GetChannel("default", "gpt-test", 0, []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"}})
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 907, channel.Id)
}

func insertChannelUserRuleCandidate(t *testing.T, id int, priority int64, rules []kitdto.ChannelUserRule) {
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
	channel.SetOtherSettings(kitdto.ChannelOtherSettings{UserRules: rules})
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
}

func TestUserRuleRPMCountsInRedisAndFallsBack(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	server := useUserCacheMiniRedis(t)
	insertChannelUserRuleCandidate(t, 921, 10, []kitdto.ChannelUserRule{{UserId: 42, RPM: 2}})
	insertChannelUserRuleCandidate(t, 922, 5, nil)
	limit := &dto.UserRuleLimit{}
	filters := []dto.ChannelFilter{{Kind: dto.FilterUserBlacklist, UserId: 42, NonStream: true, RuleLimit: limit}}

	for range 2 {
		channel, err := GetChannel("default", "gpt-test", 0, filters)
		require.NoError(t, err)
		require.NotNil(t, channel)
		require.Equal(t, 921, channel.Id)
		RecordChannelUserRuleHits(channel, "gpt-test", 42, true)
	}
	assert.Equal(t, time.Duration(channelUserRuleWindowSeconds)*time.Second, server.TTL(channelUserRuleKey(921, 0, 42)))

	channel, err := GetChannel("default", "gpt-test", 0, filters)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 922, channel.Id, "requests beyond the limit move to the next priority")
	assert.Equal(t, dto.UserRuleLimit{Limited: true, RetryAfterSeconds: channelUserRuleWindowSeconds}, *limit, "the limit reports when the window ends")

	server.FastForward(time.Duration(channelUserRuleWindowSeconds) * time.Second)
	channel, err = GetChannel("default", "gpt-test", 0, filters)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 921, channel.Id, "a new minute lets the user back on the channel")

	RecordChannelUserRuleHits(channel, "gpt-test", 42, true)
	RecordChannelUserRuleHits(channel, "gpt-test", 42, true)
	server.Close()
	channel, err = GetChannel("default", "gpt-test", 0, filters)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 921, channel.Id, "an unreadable counter lets the request use the channel")
}

func TestValidateSettingsRejectsInvalidUserRules(t *testing.T) {
	tests := []struct {
		name    string
		rules   []kitdto.ChannelUserRule
		wantErr string
	}{
		{name: "valid", rules: []kitdto.ChannelUserRule{
			{UserId: 822, Models: []string{"claude-haiku-5-5"}, NonStreamOnly: true},
			{UserId: 822, RPM: 30},
		}},
		{name: "missing user", rules: []kitdto.ChannelUserRule{{RPM: 30}}, wantErr: "user_rules[0].user_id"},
		{name: "negative rpm", rules: []kitdto.ChannelUserRule{{UserId: 822}, {UserId: 822, RPM: -1}}, wantErr: "user_rules[1].rpm"},
		{name: "blank model", rules: []kitdto.ChannelUserRule{{UserId: 822, Models: []string{" "}}}, wantErr: "user_rules[0].models"},
		{name: "space-padded model", rules: []kitdto.ChannelUserRule{{UserId: 822, Models: []string{"claude-haiku-5-5 "}}}, wantErr: "user_rules[0].models"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			channel := &Channel{Type: 1}
			channel.SetOtherSettings(kitdto.ChannelOtherSettings{UserRules: test.rules})
			err := channel.ValidateSettings()
			if test.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestUserRuleRPMLimitIsReportedOnlyForOtherwiseUsableChannels(t *testing.T) {
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })
	channel := &Channel{Id: 931, Type: 1, Models: "gpt-test"}
	channel.SetOtherSettings(kitdto.ChannelOtherSettings{UserRules: []kitdto.ChannelUserRule{{UserId: 4242, RPM: 1}}})
	RecordChannelUserRuleHits(channel, "gpt-test", 4242, false)

	limit := &dto.UserRuleLimit{}
	ok, kind := ChannelSatisfiesFilters(channel, "gpt-test", []dto.ChannelFilter{
		{Kind: dto.FilterUserBlacklist, UserId: 4242, RuleLimit: limit},
		{Kind: dto.FilterTaskPluginIdentity, TaskPluginKey: "video-plugin"},
	})
	assert.False(t, ok)
	assert.Equal(t, dto.FilterTaskPluginIdentity, kind)
	assert.False(t, limit.Limited, "waiting cannot free a channel that another filter rejects")

	ok, kind = ChannelSatisfiesFilters(channel, "gpt-test", []dto.ChannelFilter{{Kind: dto.FilterUserBlacklist, UserId: 4242, RuleLimit: limit}})
	assert.False(t, ok)
	assert.Equal(t, dto.FilterUserBlacklist, kind)
	assert.True(t, limit.Limited, "a channel only the RPM keeps the request off is reported")
}

func TestPromptAuditFallbackSkipsChannelsTheUserRulesKeepOff(t *testing.T) {
	resetChannelBlacklistTestTables(t)
	insertChannelUserRuleCandidate(t, 951, 10, []kitdto.ChannelUserRule{{UserId: 42, NonStreamOnly: true}})
	insertChannelUserRuleCandidate(t, 952, 5, nil)
	filters := []dto.ChannelFilter{
		{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"},
		{Kind: dto.FilterUserBlacklist, UserId: 42, NonStream: true},
	}
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		InitChannelCache()
	})
	for _, cached := range []bool{false, true} {
		common.MemoryCacheEnabled = cached
		InitChannelCache()

		channel, err := GetRandomUnprotectedChannel("default", "gpt-test", filters, 10, nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		assert.Equal(t, 952, channel.Id, "memory cache %t", cached)
	}
}
