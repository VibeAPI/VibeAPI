package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The user hash can expire between auth (cache hit, no TTL refresh) and wallet
// pre-consume. GetUserQuota used to surface that as "redis: nil" (HTTP 500).
func TestGetUserQuotaRehydratesAfterCacheExpiry(t *testing.T) {
	truncateTables(t)
	server := useUserCacheMiniRedis(t)

	user := User{
		Username:    "quota-cache-miss",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		Quota:       12345,
		AuthVersion: 1,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, populateUserCache(user))

	quota, err := GetUserQuota(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 12345, quota)

	server.Del(getUserCacheKey(user.Id))

	quota, err = GetUserQuota(user.Id, false)
	require.NoError(t, err, "cache miss must fall back instead of returning redis: nil")
	assert.Equal(t, 12345, quota)
	assert.True(t, server.Exists(getUserCacheKey(user.Id)), "miss should rehydrate the user cache")
}

func TestGetUserQuotaFallsBackToDBWhenRedisDown(t *testing.T) {
	truncateTables(t)
	server := useUserCacheMiniRedis(t)

	user := User{
		Username:    "quota-redis-down",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		Quota:       777,
		AuthVersion: 1,
	}
	require.NoError(t, DB.Create(&user).Error)
	server.Close()

	quota, err := GetUserQuota(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 777, quota)
}
