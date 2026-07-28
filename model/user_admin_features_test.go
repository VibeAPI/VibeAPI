package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSearchUsersByInviterIncludesInviterAndDirectInvitees(t *testing.T) {
	truncateTables(t)

	users := []*User{
		{Username: "inviter", AffCode: "inviter-code"},
		{Username: "invitee-one", AffCode: "invitee-one-code"},
		{Username: "invitee-two", AffCode: "invitee-two-code"},
		{Username: "unrelated", AffCode: "unrelated-code"},
	}
	require.NoError(t, DB.Create(&users).Error)
	require.NoError(t, DB.Model(users[1]).Update("inviter_id", users[0].Id).Error)
	require.NoError(t, DB.Model(users[2]).Update("inviter_id", users[0].Id).Error)

	result, total, err := SearchUsers("", "", nil, nil, &users[0].Id, 0, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, result, 3)
	assert.ElementsMatch(t, []int{users[0].Id, users[1].Id, users[2].Id}, []int{result[0].Id, result[1].Id, result[2].Id})
}

func TestSetManualInviterDoesNotChangeRewardCounters(t *testing.T) {
	truncateTables(t)

	inviter := &User{Username: "manual-inviter", AffCode: "manual-inviter-code"}
	invitee := &User{Username: "manual-invitee", AffCode: "manual-invitee-code"}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Create(invitee).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return SetManualInviter(tx, invitee.Id, inviter.Id)
	}))

	var updatedInviter User
	var updatedInvitee User
	require.NoError(t, DB.First(&updatedInviter, inviter.Id).Error)
	require.NoError(t, DB.First(&updatedInvitee, invitee.Id).Error)
	assert.Equal(t, inviter.Id, updatedInvitee.InviterId)
	assert.Zero(t, updatedInviter.AffCount)
	assert.Zero(t, updatedInviter.AffQuota)
	assert.Zero(t, updatedInviter.AffHistoryQuota)
}

func TestSetManualInviterRejectsCycle(t *testing.T) {
	truncateTables(t)

	first := &User{Username: "cycle-first", AffCode: "cycle-first-code"}
	second := &User{Username: "cycle-second", AffCode: "cycle-second-code"}
	require.NoError(t, DB.Create(first).Error)
	require.NoError(t, DB.Create(second).Error)
	require.NoError(t, DB.Model(first).Update("inviter_id", second.Id).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		return SetManualInviter(tx, second.Id, first.Id)
	})
	require.ErrorContains(t, err, "cycle")
}

func TestUpdateUsersRemarkUpdatesEverySelectedUser(t *testing.T) {
	truncateTables(t)

	users := []*User{
		{Username: "remark-one", AffCode: "remark-one-code"},
		{Username: "remark-two", AffCode: "remark-two-code"},
		{Username: "remark-other", AffCode: "remark-other-code"},
	}
	require.NoError(t, DB.Create(&users).Error)
	require.NoError(t, UpdateUsersRemark([]int{users[0].Id, users[1].Id}, "partner cohort"))

	var result []User
	require.NoError(t, DB.Order("id").Find(&result).Error)
	require.Len(t, result, 3)
	assert.Equal(t, "partner cohort", result[0].Remark)
	assert.Equal(t, "partner cohort", result[1].Remark)
	assert.Empty(t, result[2].Remark)
}
