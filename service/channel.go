package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason)))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

// ShouldDisableChannelForTaskFailure applies the regular automatic-disable
// rules to a terminal asynchronous task failure. Task-level interruptions such
// as "abandoned" are deliberately excluded because they do not establish that
// the channel credentials or upstream account are unhealthy.
func ShouldDisableChannelForTaskFailure(reason string, upstreamCode int) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	reason = strings.TrimSpace(reason)
	lowerReason := strings.ToLower(reason)
	if reason == "" || strings.Contains(lowerReason, "abandoned") {
		return false
	}
	for _, credentialError := range []string{"auth failed", "authentication failed", "invalid api key"} {
		if strings.Contains(lowerReason, credentialError) {
			return true
		}
	}

	statusCode := upstreamCode
	if statusCode < 100 || statusCode > 599 {
		statusCode = 0
		for _, field := range strings.FieldsFunc(reason, func(r rune) bool {
			return r < '0' || r > '9'
		}) {
			code, err := strconv.Atoi(field)
			if err == nil && code >= 100 && code <= 599 && operation_setting.ShouldDisableByStatusCode(code) {
				statusCode = code
				break
			}
		}
	}

	return ShouldDisableChannel(types.NewOpenAIError(
		fmt.Errorf("%s", reason),
		types.ErrorCodeBadResponseStatusCode,
		statusCode,
	))
}

func DisableChannelForTaskFailure(ctx context.Context, ch *model.Channel, task *model.Task, upstreamCode int) {
	if ch == nil || task == nil || !ShouldDisableChannelForTaskFailure(task.FailReason, upstreamCode) {
		return
	}
	usingKey := ""
	if ch.ChannelInfo.IsMultiKey {
		if task.PrivateData.ChannelMultiKeyIndex == nil {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s failed with a channel error, but its legacy record has no multi-key index; skip automatic disable", task.TaskID))
			return
		}
		keys := ch.GetKeys()
		keyIndex := *task.PrivateData.ChannelMultiKeyIndex
		if keyIndex < 0 || keyIndex >= len(keys) {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s has invalid multi-key index %d for channel #%d; skip automatic disable", task.TaskID, keyIndex, ch.Id))
			return
		}
		usingKey = keys[keyIndex]
		fingerprint := fmt.Sprintf("%x", common.Sha256Raw([]byte(usingKey)))
		if task.PrivateData.ChannelKeyFingerprint == "" || task.PrivateData.ChannelKeyFingerprint != fingerprint {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s multi-key fingerprint no longer matches channel #%d key index %d; skip automatic disable", task.TaskID, ch.Id, keyIndex))
			return
		}
	}
	DisableChannel(*types.NewChannelError(
		ch.Id,
		ch.Type,
		ch.Name,
		ch.ChannelInfo.IsMultiKey,
		usingKey,
		ch.GetAutoBan(),
	), fmt.Sprintf("异步任务 %s 轮询失败: %s", task.TaskID, task.FailReason))
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
