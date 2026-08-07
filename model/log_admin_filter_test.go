package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminLogFilterExcludesAdminAndRootUsers(t *testing.T) {
	truncateTables(t)

	users := []*User{
		{Username: "regular-log-user", Role: common.RoleCommonUser, AffCode: "regular-log-user"},
		{Username: "admin-log-user", Role: common.RoleAdminUser, AffCode: "admin-log-user"},
		{Username: "root-log-user", Role: common.RoleRootUser, AffCode: "root-log-user"},
	}
	require.NoError(t, DB.Create(&users).Error)

	now := time.Now().Unix()
	logs := []*Log{
		{UserId: users[0].Id, Username: users[0].Username, Type: LogTypeConsume, CreatedAt: now, Quota: 10, PromptTokens: 1},
		{UserId: users[1].Id, Username: users[1].Username, Type: LogTypeConsume, CreatedAt: now, Quota: 20, PromptTokens: 2},
		{UserId: users[2].Id, Username: users[2].Username, Type: LogTypeConsume, CreatedAt: now, Quota: 30, PromptTokens: 3},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	filteredLogs, total, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "", "", 0, 20, 0, "", "", "", true)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, filteredLogs, 1)
	assert.Equal(t, users[0].Id, filteredLogs[0].UserId)

	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", "", 0, "", true)
	require.NoError(t, err)
	assert.Equal(t, 10, stat.Quota)
	assert.Equal(t, 1, stat.Rpm)
	assert.Equal(t, 1, stat.Tpm)
}

func TestAdminLogFilterMatchesUserRemark(t *testing.T) {
	truncateTables(t)

	users := []*User{
		{Username: "partner-user", Role: common.RoleCommonUser, AffCode: "partner-user", Remark: "partner cohort"},
		{Username: "retail-user", Role: common.RoleCommonUser, AffCode: "retail-user", Remark: "retail cohort"},
	}
	require.NoError(t, DB.Create(&users).Error)

	now := time.Now().Unix()
	logs := []*Log{
		{UserId: users[0].Id, Username: users[0].Username, Type: LogTypeConsume, CreatedAt: now, Quota: 10},
		{UserId: users[1].Id, Username: users[1].Username, Type: LogTypeConsume, CreatedAt: now, Quota: 20},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	filteredLogs, total, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "partner", "", 0, 20, 0, "", "", "", false)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, filteredLogs, 1)
	assert.Equal(t, users[0].Id, filteredLogs[0].UserId)

	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "", "partner", "", 0, "", false)
	require.NoError(t, err)
	assert.Equal(t, 10, stat.Quota)
}

func TestAdminLogFilterIgnoresNonPositiveChannelSentinel(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()
	logs := []*Log{
		{Type: LogTypeConsume, CreatedAt: now, ChannelId: 11, Quota: 10},
		{Type: LogTypeConsume, CreatedAt: now, ChannelId: 22, Quota: 20},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	filteredLogs, total, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "", "", 0, 20, -1, "", "", "", false)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, filteredLogs, 2)

	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", "", -1, "", false)
	require.NoError(t, err)
	assert.Equal(t, 30, stat.Quota)
}

func TestGetTopupLogsForExportFiltersTimeRangeAndAdministrators(t *testing.T) {
	truncateTables(t)

	users := []*User{
		{Username: "topup-export-user", Role: common.RoleCommonUser, AffCode: "topup-export-user"},
		{Username: "topup-export-admin", Role: common.RoleAdminUser, AffCode: "topup-export-admin"},
	}
	require.NoError(t, DB.Create(&users).Error)

	logs := []*Log{
		{UserId: users[0].Id, Username: users[0].Username, Type: LogTypeTopup, CreatedAt: 100, Quota: 500_000},
		{UserId: users[0].Id, Username: users[0].Username, Type: LogTypeTopup, CreatedAt: 200, Quota: 1_000_000, Content: "充值成功，支付金额：1.60"},
		{UserId: users[1].Id, Username: users[1].Username, Type: LogTypeTopup, CreatedAt: 250, Quota: 1_500_000},
		{UserId: users[0].Id, Username: users[0].Username, Type: LogTypeConsume, CreatedAt: 275, Quota: 1},
		{UserId: users[0].Id, Username: users[0].Username, Type: LogTypeTopup, CreatedAt: 300, Quota: 2_000_000},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	exported, err := GetTopupLogsForExport(150, 275, true)
	require.NoError(t, err)
	require.Len(t, exported, 1)
	assert.Equal(t, users[0].Username, exported[0].Username)
	assert.Equal(t, int64(200), exported[0].CreatedAt)
	assert.Equal(t, 1_000_000, exported[0].Quota)
	assert.Equal(t, LogTypeTopup, exported[0].Type)
	amount, ok := GetTopupPaymentAmount(exported[0])
	require.True(t, ok)
	assert.Equal(t, 1.6, amount)
}
