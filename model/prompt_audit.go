package model

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	PromptAuditEventPending   = "pending"
	PromptAuditEventSafe      = "safe"
	PromptAuditEventViolation = "violation"
	PromptAuditEventFailed    = "failed"
)

type PromptAuditEvent struct {
	Id                 int64   `json:"id"`
	EventId            string  `json:"event_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId             int     `json:"user_id" gorm:"index;not null"`
	RequestId          string  `json:"request_id" gorm:"type:varchar(64);index"`
	ContentFingerprint string  `json:"content_fingerprint" gorm:"type:varchar(64);index;not null"`
	ConfigVersion      int64   `json:"config_version" gorm:"index;not null"`
	Status             string  `json:"status" gorm:"type:varchar(32);index;not null"`
	Categories         string  `json:"categories" gorm:"type:text"`
	MainConfidence     float64 `json:"main_confidence"`
	ValidVotes         int     `json:"valid_votes"`
	FlaggedVotes       int     `json:"flagged_votes"`
	Votes              string  `json:"votes" gorm:"type:text"`
	OriginChannelId    int     `json:"origin_channel_id"`
	FinalChannelId     int     `json:"final_channel_id"`
	ContentTruncated   bool    `json:"content_truncated"`
	FailureType        string  `json:"failure_type" gorm:"type:varchar(64)"`
	LatencyMs          int64   `json:"latency_ms"`
	EmailStatus        string  `json:"email_status" gorm:"type:varchar(32)"`
	EmailError         string  `json:"email_error" gorm:"type:varchar(512)"`
	EmailAttempts      int     `json:"email_attempts"`
	CreatedAt          int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt          int64   `json:"updated_at" gorm:"bigint"`
}

type PromptAuditUserState struct {
	UserId              int    `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	PendingEventId      string `json:"pending_event_id" gorm:"type:varchar(64);index"`
	RestrictedUntil     int64  `json:"restricted_until" gorm:"bigint;index"`
	IndefinitelyBlocked bool   `json:"indefinitely_blocked"`
	ViolationCount      int    `json:"violation_count"`
	LastViolationAt     int64  `json:"last_violation_at" gorm:"bigint"`
	UpdatedAt           int64  `json:"updated_at" gorm:"bigint"`
}

type PromptAuditAdminAction struct {
	Id         int64  `json:"id"`
	UserId     int    `json:"user_id" gorm:"index"`
	OperatorId int    `json:"operator_id" gorm:"index"`
	Action     string `json:"action" gorm:"type:varchar(64)"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
}

func (event *PromptAuditEvent) BeforeCreate(_ *gorm.DB) error {
	now := time.Now().Unix()
	if event.CreatedAt == 0 {
		event.CreatedAt = now
	}
	event.UpdatedAt = now
	return nil
}

func (state *PromptAuditUserState) BeforeSave(_ *gorm.DB) error {
	state.UpdatedAt = time.Now().Unix()
	return nil
}

func FindRecentPromptAuditEvent(userId int, fingerprint string, version int64, since int64) (*PromptAuditEvent, error) {
	var event PromptAuditEvent
	err := DB.Where("user_id = ? AND content_fingerprint = ? AND config_version = ? AND created_at >= ?", userId, fingerprint, version, since).
		Order("id DESC").First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &event, err
}

func GetPromptAuditEvent(eventId string) (*PromptAuditEvent, error) {
	var event PromptAuditEvent
	if err := DB.Where("event_id = ?", eventId).First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func CreatePromptAuditEventAndMarkPending(event *PromptAuditEvent) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		state := PromptAuditUserState{UserId: event.UserId}
		if err := tx.FirstOrCreate(&state, PromptAuditUserState{UserId: event.UserId}).Error; err != nil {
			return err
		}
		return tx.Model(&PromptAuditUserState{}).Where("user_id = ?", event.UserId).Updates(map[string]any{
			"pending_event_id": event.EventId,
			"updated_at":       time.Now().Unix(),
		}).Error
	})
}

func GetPromptAuditUserState(userId int) (*PromptAuditUserState, error) {
	var state PromptAuditUserState
	err := DB.Where("user_id = ?", userId).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &state, err
}

func FinishPromptAuditEvent(eventId string, status string, categories string, validVotes int, flaggedVotes int, votes string, failureType string, latencyMs int64, restrictionHours int, resetDays int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var event PromptAuditEvent
		if err := lockForUpdate(tx).Where("event_id = ? AND status = ?", eventId, PromptAuditEventPending).First(&event).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		updates := map[string]any{
			"status": status, "categories": categories, "valid_votes": validVotes,
			"flagged_votes": flaggedVotes, "votes": votes, "failure_type": failureType,
			"latency_ms": latencyMs, "updated_at": now,
		}
		if err := tx.Model(&event).Updates(updates).Error; err != nil {
			return err
		}
		var state PromptAuditUserState
		if err := lockForUpdate(tx).Where("user_id = ?", event.UserId).First(&state).Error; err != nil {
			return err
		}
		if state.PendingEventId == eventId {
			state.PendingEventId = ""
		}
		if status == PromptAuditEventViolation {
			if resetDays > 0 && state.LastViolationAt > 0 && now-state.LastViolationAt >= int64(resetDays*86400) {
				state.ViolationCount = 0
			}
			state.ViolationCount++
			state.LastViolationAt = now
			if state.ViolationCount >= 3 || restrictionHours <= 0 {
				state.IndefinitelyBlocked = true
				state.RestrictedUntil = 0
			} else {
				state.IndefinitelyBlocked = false
				state.RestrictedUntil = now + int64(restrictionHours*3600)
			}
		}
		return tx.Save(&state).Error
	})
}

func ListPromptAuditEvents(status string, userId int, page int, pageSize int) ([]PromptAuditEvent, int64, error) {
	query := DB.Model(&PromptAuditEvent{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var events []PromptAuditEvent
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func UpdatePromptAuditEventEmail(eventId string, status string, attempts int, message string) error {
	return DB.Model(&PromptAuditEvent{}).Where("event_id = ?", eventId).Updates(map[string]any{
		"email_status": status, "email_attempts": attempts, "email_error": message, "updated_at": time.Now().Unix(),
	}).Error
}

func UpdatePromptAuditEventFinalChannel(eventId string, channelId int) error {
	if strings.TrimSpace(eventId) == "" || channelId <= 0 {
		return nil
	}
	return DB.Model(&PromptAuditEvent{}).Where("event_id = ?", eventId).Updates(map[string]any{
		"final_channel_id": channelId, "updated_at": time.Now().Unix(),
	}).Error
}

// CancelPendingPromptAuditEvents safely resolves in-flight reviews when the
// operator disables auditing or changes configuration. They must not later
// transition a user into a restriction.
func CancelPendingPromptAuditEvents() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		if err := tx.Model(&PromptAuditEvent{}).Where("status = ?", PromptAuditEventPending).Updates(map[string]any{
			"status": PromptAuditEventFailed, "failure_type": "audit_disabled", "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&PromptAuditUserState{}).Where("pending_event_id <> ''").Updates(map[string]any{
			"pending_event_id": "", "updated_at": now,
		}).Error
	})
}

func FailStalePromptAuditEvents(before int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var events []PromptAuditEvent
		if err := tx.Where("status = ? AND created_at < ?", PromptAuditEventPending, before).Find(&events).Error; err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		now := time.Now().Unix()
		eventIDs := make([]string, 0, len(events))
		for _, event := range events {
			eventIDs = append(eventIDs, event.EventId)
		}
		if err := tx.Model(&PromptAuditEvent{}).Where("event_id IN ?", eventIDs).Updates(map[string]any{
			"status": PromptAuditEventFailed, "failure_type": "worker_interrupted", "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&PromptAuditUserState{}).Where("pending_event_id IN ?", eventIDs).Updates(map[string]any{
			"pending_event_id": "", "updated_at": now,
		}).Error
	})
}

func ClearPromptAuditRestriction(userId int, resetCount bool) error {
	updates := map[string]any{"pending_event_id": "", "restricted_until": 0, "indefinitely_blocked": false, "updated_at": time.Now().Unix()}
	if resetCount {
		updates["violation_count"] = 0
		updates["last_violation_at"] = 0
	}
	return DB.Model(&PromptAuditUserState{}).Where("user_id = ?", userId).Updates(updates).Error
}

func CleanupPromptAuditEvents(retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("created_at < ?", cutoff).Delete(&PromptAuditEvent{}).Error; err != nil {
			return err
		}
		return tx.Where("created_at < ?", cutoff).Delete(&PromptAuditAdminAction{}).Error
	})
}

func CreatePromptAuditAdminAction(userId int, operatorId int, action string) error {
	if userId <= 0 || operatorId <= 0 || action == "" {
		return errors.New("invalid prompt audit admin action")
	}
	return DB.Create(&PromptAuditAdminAction{UserId: userId, OperatorId: operatorId, Action: action, CreatedAt: time.Now().Unix()}).Error
}

func PromptAuditFingerprint(userId int, content string, version int64) string {
	return common.GenerateHMAC(strconv.Itoa(userId) + "\x00" + content + "\x00" + strconv.FormatInt(version, 10))
}
