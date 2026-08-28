package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

const maxPromptAuditSettingsBodySize = 512 << 10

type promptAuditEndpointInput struct {
	URL            string `json:"url"`
	APIKey         string `json:"api_key"`
	HasAPIKey      bool   `json:"has_api_key"`
	Model          string `json:"model"`
	SystemPrompt   string `json:"system_prompt"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type promptAuditSettingsInput struct {
	Enabled                   bool                     `json:"enabled"`
	Mode                      string                   `json:"mode"`
	ProtectedChannelIds       []int                    `json:"protected_channel_ids"`
	AudienceMode              string                   `json:"audience_mode"`
	AudienceUserIds           []int                    `json:"audience_user_ids"`
	ContentScope              string                   `json:"content_scope"`
	MaxCharacters             int                      `json:"max_characters"`
	MainThreshold             float64                  `json:"main_threshold"`
	ReviewThreshold           float64                  `json:"review_threshold"`
	ReviewEnabled             bool                     `json:"review_enabled"`
	RequiredValidVotes        int                      `json:"required_valid_votes"`
	RequiredFlaggedVotes      int                      `json:"required_flagged_votes"`
	ReviewTotalTimeoutSeconds int                      `json:"review_total_timeout_seconds"`
	AllowPrivateEndpoints     bool                     `json:"allow_private_endpoints"`
	FirstRestrictionHours     int                      `json:"first_restriction_hours"`
	SecondRestrictionHours    int                      `json:"second_restriction_hours"`
	ViolationResetDays        int                      `json:"violation_reset_days"`
	DedupeMinutes             int                      `json:"dedupe_minutes"`
	RetentionDays             int                      `json:"retention_days"`
	RejectMessage             string                   `json:"reject_message"`
	AppealContact             string                   `json:"appeal_contact"`
	Version                   int64                    `json:"version"`
	TestedVersion             int64                    `json:"tested_version"`
	MainTestedVersion         int64                    `json:"main_tested_version"`
	ReviewTestedVersion       int64                    `json:"review_tested_version"`
	Main                      promptAuditEndpointInput `json:"main"`
	Review                    promptAuditEndpointInput `json:"review"`
}

type promptAuditTestRequest struct {
	Stage   string `json:"stage"`
	Content string `json:"content"`
}

func promptAuditSettingsResponse(setting operation_setting.PromptAuditSetting) promptAuditSettingsInput {
	return promptAuditSettingsInput{
		Enabled: setting.Enabled, Mode: setting.Mode, ProtectedChannelIds: setting.ProtectedChannelIds,
		AudienceMode: setting.AudienceMode, AudienceUserIds: setting.AudienceUserIds, ContentScope: setting.ContentScope,
		MaxCharacters: setting.MaxCharacters, MainThreshold: setting.MainThreshold, ReviewThreshold: setting.ReviewThreshold, ReviewEnabled: setting.ReviewEnabled,
		RequiredValidVotes: setting.RequiredValidVotes, RequiredFlaggedVotes: setting.RequiredFlaggedVotes,
		ReviewTotalTimeoutSeconds: setting.ReviewTotalTimeoutSeconds, AllowPrivateEndpoints: setting.AllowPrivateEndpoints,
		FirstRestrictionHours: setting.FirstRestrictionHours, SecondRestrictionHours: setting.SecondRestrictionHours,
		ViolationResetDays: setting.ViolationResetDays, DedupeMinutes: setting.DedupeMinutes, RetentionDays: setting.RetentionDays,
		RejectMessage: setting.RejectMessage, AppealContact: setting.AppealContact, Version: setting.Version, TestedVersion: setting.TestedVersion,
		MainTestedVersion: setting.MainTestedVersion, ReviewTestedVersion: setting.ReviewTestedVersion,
		Main:   promptAuditEndpointInput{URL: setting.Main.URL, HasAPIKey: setting.Main.APIKeyEncrypted != "", Model: setting.Main.Model, SystemPrompt: setting.Main.SystemPrompt, TimeoutSeconds: setting.Main.TimeoutSeconds},
		Review: promptAuditEndpointInput{URL: setting.Review.URL, HasAPIKey: setting.Review.APIKeyEncrypted != "" || setting.Main.APIKeyEncrypted != "", Model: setting.Review.Model, SystemPrompt: setting.Review.SystemPrompt, TimeoutSeconds: setting.Review.TimeoutSeconds},
	}
}

func GetPromptAuditSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, promptAuditSettingsResponse(service.SnapshotPromptAuditSetting()))
}

func SavePromptAuditSettings(c *gin.Context) {
	var input promptAuditSettingsInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPromptAuditSettingsBodySize)
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		common.ApiErrorMsg(c, "Invalid prompt audit settings")
		return
	}
	current := service.SnapshotPromptAuditSetting()
	setting, err := validatePromptAuditSettingsInput(input, current)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	setting.Version = current.Version + 1
	setting.TestedVersion = 0
	setting.MainTestedVersion = 0
	setting.ReviewTestedVersion = 0
	setting.Enabled = false
	values, err := config.ConfigToMap(&setting)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	updates := make(map[string]string, len(values))
	for key, value := range values {
		updates["prompt_audit_setting."+key] = value
	}
	if err := model.UpdateOptionsBulk(updates); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.CancelPendingPromptAuditEvents(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "prompt_audit.update", map[string]interface{}{"version": setting.Version, "requested_enabled": input.Enabled})
	common.ApiSuccess(c, gin.H{"version": setting.Version, "requires_test": true})
}

func validatePromptAuditSettingsInput(input promptAuditSettingsInput, current operation_setting.PromptAuditSetting) (operation_setting.PromptAuditSetting, error) {
	if input.Mode != operation_setting.PromptAuditModeDowngrade && input.Mode != operation_setting.PromptAuditModeReject {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("invalid prompt audit mode")
	}
	if input.AudienceMode != operation_setting.PromptAuditScopeAll && input.AudienceMode != operation_setting.PromptAuditScopeWhitelist && input.AudienceMode != operation_setting.PromptAuditScopeBlacklist {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("invalid audit audience mode")
	}
	if input.ContentScope != operation_setting.PromptAuditContentLatest && input.ContentScope != operation_setting.PromptAuditContentLatestTools && input.ContentScope != operation_setting.PromptAuditContentAll {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("invalid audit content scope")
	}
	if len(input.ProtectedChannelIds) == 0 || len(input.ProtectedChannelIds) > 1000 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("select between 1 and 1000 protected channels")
	}
	if len(input.AudienceUserIds) > 10000 || input.MaxCharacters < 1 || input.MaxCharacters > 200000 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("audit audience or content limit is out of range")
	}
	if input.MainThreshold < 0 || input.MainThreshold > 1 || input.ReviewThreshold < 0 || input.ReviewThreshold > 1 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("audit thresholds must be between 0 and 1")
	}
	if input.ReviewEnabled && (input.RequiredValidVotes < 1 || input.RequiredValidVotes > 5 || input.RequiredFlaggedVotes < 1 || input.RequiredFlaggedVotes > input.RequiredValidVotes) {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("review vote thresholds are invalid")
	}
	if input.ReviewTotalTimeoutSeconds < 1 || input.ReviewTotalTimeoutSeconds > 120 || input.Main.TimeoutSeconds < 1 || input.Main.TimeoutSeconds > 60 || input.Review.TimeoutSeconds < 1 || input.Review.TimeoutSeconds > 60 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("audit timeouts are out of range")
	}
	if input.FirstRestrictionHours < 1 || input.FirstRestrictionHours > 8760 || input.SecondRestrictionHours < input.FirstRestrictionHours || input.SecondRestrictionHours > 87600 || input.ViolationResetDays < 1 || input.ViolationResetDays > 3650 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("restriction settings are invalid")
	}
	if input.DedupeMinutes < 1 || input.DedupeMinutes > 1440 || input.RetentionDays < 7 || input.RetentionDays > 365 || len([]rune(input.RejectMessage)) > 500 || len([]rune(input.AppealContact)) > 500 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("retention, deduplication, or message settings are invalid")
	}
	mainEndpoint, err := promptAuditEndpointFromInput(input.Main, current.Main, input.AllowPrivateEndpoints, false)
	if err != nil {
		return operation_setting.PromptAuditSetting{}, err
	}
	reviewEndpoint := current.Review
	if input.ReviewEnabled {
		reviewEndpoint, err = promptAuditEndpointFromInput(input.Review, current.Review, input.AllowPrivateEndpoints, true)
		if err != nil {
			return operation_setting.PromptAuditSetting{}, err
		}
	}
	seenChannels := make(map[int]struct{}, len(input.ProtectedChannelIds))
	for _, channelId := range input.ProtectedChannelIds {
		if channelId <= 0 {
			return operation_setting.PromptAuditSetting{}, fmt.Errorf("protected channel IDs must be positive")
		}
		if _, exists := seenChannels[channelId]; exists {
			return operation_setting.PromptAuditSetting{}, fmt.Errorf("protected channel IDs must be unique")
		}
		seenChannels[channelId] = struct{}{}
		if _, err := model.CacheGetChannel(channelId); err != nil {
			return operation_setting.PromptAuditSetting{}, fmt.Errorf("protected channel %d does not exist", channelId)
		}
	}
	return operation_setting.PromptAuditSetting{
		Enabled: false, Mode: input.Mode, ProtectedChannelIds: input.ProtectedChannelIds,
		AudienceMode: input.AudienceMode, AudienceUserIds: input.AudienceUserIds, ContentScope: input.ContentScope,
		MaxCharacters: input.MaxCharacters, MainThreshold: input.MainThreshold, ReviewThreshold: input.ReviewThreshold, ReviewEnabled: input.ReviewEnabled,
		RequiredValidVotes: input.RequiredValidVotes, RequiredFlaggedVotes: input.RequiredFlaggedVotes,
		ReviewTotalTimeoutSeconds: input.ReviewTotalTimeoutSeconds, AllowPrivateEndpoints: input.AllowPrivateEndpoints,
		FirstRestrictionHours: input.FirstRestrictionHours, SecondRestrictionHours: input.SecondRestrictionHours,
		ViolationResetDays: input.ViolationResetDays, DedupeMinutes: input.DedupeMinutes, RetentionDays: input.RetentionDays,
		RejectMessage: strings.TrimSpace(input.RejectMessage), AppealContact: strings.TrimSpace(input.AppealContact),
		Main: mainEndpoint, Review: reviewEndpoint,
	}, nil
}

func promptAuditEndpointFromInput(input promptAuditEndpointInput, current operation_setting.PromptAuditEndpoint, allowPrivate bool, mayInherit bool) (operation_setting.PromptAuditEndpoint, error) {
	input.URL = strings.TrimSpace(input.URL)
	input.Model = strings.TrimSpace(input.Model)
	input.SystemPrompt = strings.TrimSpace(input.SystemPrompt)
	if input.URL == "" && !mayInherit {
		return operation_setting.PromptAuditEndpoint{}, fmt.Errorf("main audit URL is required")
	}
	if input.URL != "" {
		if err := service.ValidatePromptAuditEndpoint(input.URL, allowPrivate); err != nil {
			return operation_setting.PromptAuditEndpoint{}, err
		}
	}
	if input.Model == "" || input.SystemPrompt == "" || len([]rune(input.SystemPrompt)) > 20000 {
		return operation_setting.PromptAuditEndpoint{}, fmt.Errorf("audit model and system prompt are required")
	}
	encryptedKey := current.APIKeyEncrypted
	if strings.TrimSpace(input.APIKey) != "" {
		var err error
		encryptedKey, err = common.EncryptSecret(strings.TrimSpace(input.APIKey))
		if err != nil {
			return operation_setting.PromptAuditEndpoint{}, err
		}
	}
	if encryptedKey == "" && !mayInherit {
		return operation_setting.PromptAuditEndpoint{}, fmt.Errorf("main audit API key is required")
	}
	return operation_setting.PromptAuditEndpoint{URL: input.URL, APIKeyEncrypted: encryptedKey, Model: input.Model, SystemPrompt: input.SystemPrompt, TimeoutSeconds: input.TimeoutSeconds}, nil
}

func TestPromptAuditSettings(c *gin.Context) {
	var request promptAuditTestRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "Invalid test request")
		return
	}
	setting := service.SnapshotPromptAuditSetting()
	if request.Stage != "main" && request.Stage != "review" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "stage must be main or review"})
		return
	}
	content := strings.TrimSpace(request.Content)
	if content == "" {
		content = "Explain how to improve account security without bypassing safeguards."
	}
	testCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	var decision service.PromptAuditDecision
	var duration time.Duration
	var err error
	if request.Stage == "review" {
		decision, duration, err = service.RunPromptAuditReview(testCtx, setting, content, service.PromptAuditDecision{Flagged: false, Confidence: 0.1, Categories: []string{}})
	} else {
		decision, duration, err = service.RunPromptAuditMain(testCtx, setting, content)
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	updates := map[string]string{}
	if request.Stage == "main" {
		updates["prompt_audit_setting.main_tested_version"] = fmt.Sprintf("%d", setting.Version)
		setting.MainTestedVersion = setting.Version
	} else {
		updates["prompt_audit_setting.review_tested_version"] = fmt.Sprintf("%d", setting.Version)
		setting.ReviewTestedVersion = setting.Version
	}
	if setting.MainTestedVersion == setting.Version && (!setting.ReviewEnabled || setting.ReviewTestedVersion == setting.Version) {
		updates["prompt_audit_setting.tested_version"] = fmt.Sprintf("%d", setting.Version)
	}
	if err := model.UpdateOptionsBulk(updates); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"decision": decision, "latency_ms": duration.Milliseconds(), "version": setting.Version})
}

func EnablePromptAudit(c *gin.Context) {
	setting := service.SnapshotPromptAuditSetting()
	if setting.MainTestedVersion != setting.Version || (setting.ReviewEnabled && setting.ReviewTestedVersion != setting.Version) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Run all enabled audit-stage connection tests before enabling prompt audit"})
		return
	}
	// Clear any orphaned pending markers before enforcement resumes.
	if err := model.FailStalePromptAuditEvents(time.Now().Unix() + 1); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOption("prompt_audit_setting.enabled", "true"); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "prompt_audit.enable", map[string]interface{}{"version": setting.Version})
	common.ApiSuccess(c, nil)
}

func DisablePromptAudit(c *gin.Context) {
	if err := model.UpdateOption("prompt_audit_setting.enabled", "false"); err != nil {
		common.ApiError(c, err)
		return
	}
	// Resolve any in-flight events before returning so late review goroutines
	// cannot apply restrictions after the global switch is turned off.
	if err := model.CancelPendingPromptAuditEvents(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "prompt_audit.disable", nil)
	common.ApiSuccess(c, nil)
}

func ListPromptAuditEvents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	page, pageSize := pageInfo.GetPage(), pageInfo.GetPageSize()
	status := strings.TrimSpace(c.Query("status"))
	userId := c.GetInt("target_user_id")
	if raw := c.Query("user_id"); raw != "" {
		_, _ = fmt.Sscanf(raw, "%d", &userId)
	}
	events, total, err := model.ListPromptAuditEvents(status, userId, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": events, "total": total, "page": page, "page_size": pageSize})
}

func ClearPromptAuditUserRestriction(c *gin.Context) {
	var request struct {
		UserId     int  `json:"user_id"`
		ResetCount bool `json:"reset_count"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.UserId <= 0 {
		common.ApiErrorMsg(c, "Invalid user restriction request")
		return
	}
	if err := model.ClearPromptAuditRestriction(request.UserId, request.ResetCount); err != nil {
		common.ApiError(c, err)
		return
	}
	_ = model.CreatePromptAuditAdminAction(request.UserId, c.GetInt("id"), "restriction_cleared")
	recordManageAudit(c, "prompt_audit.restriction_clear", map[string]interface{}{"user_id": request.UserId, "reset_count": request.ResetCount})
	common.ApiSuccess(c, nil)
}

func ResendPromptAuditEmail(c *gin.Context) {
	eventId := strings.TrimSpace(c.Param("event_id"))
	setting := service.SnapshotPromptAuditSetting()
	if err := model.UpdatePromptAuditEventEmail(eventId, "pending", 0, ""); err != nil {
		common.ApiError(c, err)
		return
	}
	go service.SendPromptAuditNotification(eventId, setting)
	recordManageAudit(c, "prompt_audit.email_resend", map[string]interface{}{"event_id": eventId})
	common.ApiSuccess(c, nil)
}
