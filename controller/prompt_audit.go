package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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
	Enabled               bool                     `json:"enabled"`
	Mode                  string                   `json:"mode"`
	ProtectedChannelIds   []int                    `json:"protected_channel_ids"`
	AudienceUserIds       []int                    `json:"audience_user_ids"`
	ContentScope          string                   `json:"content_scope"`
	MaxCharacters         int                      `json:"max_characters"`
	MainThreshold         float64                  `json:"main_threshold"`
	AllowPrivateEndpoints bool                     `json:"allow_private_endpoints"`
	RejectMessage         string                   `json:"reject_message"`
	Version               int64                    `json:"version"`
	TestedVersion         int64                    `json:"tested_version"`
	MainTestedVersion     int64                    `json:"main_tested_version"`
	Main                  promptAuditEndpointInput `json:"main"`
}

type promptAuditSettingsResponseData struct {
	promptAuditSettingsInput
	RestrictedUsers       []model.ChannelBlacklistUser `json:"restricted_users"`
	ProtectedChannelCount int                          `json:"protected_channel_count"`
}

type promptAuditTestRequest struct {
	Stage   string `json:"stage"`
	Content string `json:"content"`
}

func promptAuditSettingsResponse(setting operation_setting.PromptAuditSetting) promptAuditSettingsInput {
	return promptAuditSettingsInput{
		Enabled: setting.Enabled, Mode: setting.Mode, ProtectedChannelIds: append([]int{}, setting.ProtectedChannelIds...),
		AudienceUserIds: append([]int{}, setting.AudienceUserIds...), ContentScope: setting.ContentScope,
		MaxCharacters: setting.MaxCharacters, MainThreshold: setting.MainThreshold, AllowPrivateEndpoints: setting.AllowPrivateEndpoints,
		RejectMessage: setting.RejectMessage, Version: setting.Version, TestedVersion: setting.TestedVersion,
		MainTestedVersion: setting.MainTestedVersion,
		Main:              promptAuditEndpointInput{URL: setting.Main.URL, HasAPIKey: setting.Main.APIKeyEncrypted != "", Model: setting.Main.Model, SystemPrompt: setting.Main.SystemPrompt, TimeoutSeconds: setting.Main.TimeoutSeconds},
	}
}

func GetPromptAuditSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	setting := service.SnapshotPromptAuditSetting()
	users, err := model.ListChannelBlacklistUsers(setting.ProtectedChannelIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, promptAuditSettingsResponseData{
		promptAuditSettingsInput: promptAuditSettingsResponse(setting),
		RestrictedUsers:          users,
		ProtectedChannelCount:    len(setting.ProtectedChannelIds),
	})
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
	recordManageAudit(c, "prompt_audit.update", map[string]interface{}{"version": setting.Version, "requested_enabled": input.Enabled})
	common.ApiSuccess(c, gin.H{"version": setting.Version, "requires_test": true})
}

func validatePromptAuditSettingsInput(input promptAuditSettingsInput, current operation_setting.PromptAuditSetting) (operation_setting.PromptAuditSetting, error) {
	if input.Mode != operation_setting.PromptAuditModeDowngrade && input.Mode != operation_setting.PromptAuditModeReject {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("invalid prompt audit mode")
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
	if input.MainThreshold < 0 || input.MainThreshold > 1 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("audit thresholds must be between 0 and 1")
	}
	if input.Main.TimeoutSeconds < 1 || input.Main.TimeoutSeconds > 60 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("audit timeouts are out of range")
	}
	if len([]rune(input.RejectMessage)) > 500 {
		return operation_setting.PromptAuditSetting{}, fmt.Errorf("rejection message is too long")
	}
	mainEndpoint, err := promptAuditEndpointFromInput(input.Main, current.Main, input.AllowPrivateEndpoints, false)
	if err != nil {
		return operation_setting.PromptAuditSetting{}, err
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
		AudienceUserIds: input.AudienceUserIds, ContentScope: input.ContentScope,
		MaxCharacters: input.MaxCharacters, MainThreshold: input.MainThreshold,
		AllowPrivateEndpoints: input.AllowPrivateEndpoints,
		RejectMessage:         strings.TrimSpace(input.RejectMessage),
		Main:                  mainEndpoint,
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
	if request.Stage != "" && request.Stage != "main" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "stage must be main"})
		return
	}
	content := strings.TrimSpace(request.Content)
	if content == "" {
		content = "Explain how to improve account security without bypassing safeguards."
	}
	testCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	decision, duration, err := service.RunPromptAuditMain(testCtx, setting, content)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	updates := map[string]string{
		"prompt_audit_setting.main_tested_version": fmt.Sprintf("%d", setting.Version),
		"prompt_audit_setting.tested_version":      fmt.Sprintf("%d", setting.Version),
	}
	if err := model.UpdateOptionsBulk(updates); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"decision": decision, "latency_ms": duration.Milliseconds(), "version": setting.Version})
}

func EnablePromptAudit(c *gin.Context) {
	setting := service.SnapshotPromptAuditSetting()
	if setting.MainTestedVersion != setting.Version {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Run the audit connection test before enabling prompt audit"})
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
	recordManageAudit(c, "prompt_audit.disable", nil)
	common.ApiSuccess(c, nil)
}

func ListPromptAuditBlacklist(c *gin.Context) {
	setting := service.SnapshotPromptAuditSetting()
	users, err := model.ListChannelBlacklistUsers(setting.ProtectedChannelIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": users, "total": len(users), "channel_count": len(setting.ProtectedChannelIds)})
}

func RemovePromptAuditBlacklistUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "Invalid blacklist request")
		return
	}
	setting := service.SnapshotPromptAuditSetting()
	if err := model.RemoveUserFromChannelBlacklists(setting.ProtectedChannelIds, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "prompt_audit.blacklist_remove", map[string]interface{}{"user_id": userId, "channel_ids": setting.ProtectedChannelIds})
	common.ApiSuccess(c, nil)
}
