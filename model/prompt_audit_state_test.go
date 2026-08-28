package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createPromptAuditStateEvent(t *testing.T, userID int, suffix string) *PromptAuditEvent {
	t.Helper()
	event := &PromptAuditEvent{
		EventId: fmt.Sprintf("audit-%d-%s", userID, suffix), UserId: userID,
		ContentFingerprint: fmt.Sprintf("fingerprint-%s", suffix), ConfigVersion: 1,
		Status: PromptAuditEventPending, EmailStatus: "not_required",
	}
	require.NoError(t, CreatePromptAuditEventAndMarkPending(event))
	return event
}

func TestPromptAuditViolationRestrictionScheduleAndReset(t *testing.T) {
	truncateTables(t)
	const userID = 9101

	first := createPromptAuditStateEvent(t, userID, "first")
	require.NoError(t, FinishPromptAuditEvent(first.EventId, PromptAuditEventViolation, "[]", 5, 5, "[]", "", 10, 24, 90))
	state, err := GetPromptAuditUserState(userID)
	require.NoError(t, err)
	assert.Equal(t, 1, state.ViolationCount)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), time.Unix(state.RestrictedUntil, 0), 3*time.Second)

	second := createPromptAuditStateEvent(t, userID, "second")
	require.NoError(t, FinishPromptAuditEvent(second.EventId, PromptAuditEventViolation, "[]", 5, 5, "[]", "", 10, 168, 90))
	state, err = GetPromptAuditUserState(userID)
	require.NoError(t, err)
	assert.Equal(t, 2, state.ViolationCount)
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), time.Unix(state.RestrictedUntil, 0), 3*time.Second)

	third := createPromptAuditStateEvent(t, userID, "third")
	require.NoError(t, FinishPromptAuditEvent(third.EventId, PromptAuditEventViolation, "[]", 5, 5, "[]", "", 10, 168, 90))
	state, err = GetPromptAuditUserState(userID)
	require.NoError(t, err)
	assert.Equal(t, 3, state.ViolationCount)
	assert.True(t, state.IndefinitelyBlocked)

	require.NoError(t, DB.Model(&PromptAuditUserState{}).Where("user_id = ?", userID).Updates(map[string]any{
		"violation_count": 2, "last_violation_at": time.Now().Add(-91 * 24 * time.Hour).Unix(),
	}).Error)
	fourth := createPromptAuditStateEvent(t, userID, "reset")
	require.NoError(t, FinishPromptAuditEvent(fourth.EventId, PromptAuditEventViolation, "[]", 5, 5, "[]", "", 10, 24, 90))
	state, err = GetPromptAuditUserState(userID)
	require.NoError(t, err)
	assert.Equal(t, 1, state.ViolationCount)
	assert.False(t, state.IndefinitelyBlocked)
}

func TestCancelPendingPromptAuditEventsPreventsLateTransition(t *testing.T) {
	truncateTables(t)
	event := createPromptAuditStateEvent(t, 9102, "cancel")

	require.NoError(t, CancelPendingPromptAuditEvents())
	state, err := GetPromptAuditUserState(event.UserId)
	require.NoError(t, err)
	assert.Empty(t, state.PendingEventId)
	assert.Equal(t, 0, state.ViolationCount)

	err = FinishPromptAuditEvent(event.EventId, PromptAuditEventViolation, "[]", 5, 5, "[]", "", 10, 24, 90)
	assert.Error(t, err)
	reloaded, err := GetPromptAuditEvent(event.EventId)
	require.NoError(t, err)
	assert.Equal(t, PromptAuditEventFailed, reloaded.Status)
	assert.Equal(t, "audit_disabled", reloaded.FailureType)
}
