package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/requestrule"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// DecideRelayRetry is the single retry decision for relay attempts. The reason
// is recorded in the request policy decision events of the log details.
func DecideRelayRetry(c *gin.Context, err *types.NewAPIError, retryTimes int) PolicyDecision {
	if err == nil {
		return PolicyDecision{Action: "stop", Reason: "request_completed", Source: "system"}
	}
	if ShouldSkipRetryAfterChannelAffinityFailure(c) {
		source := RequestPolicy(c).SessionModeSource
		if source == "" {
			source = "session_rule"
		}
		return PolicyDecision{Action: "stop", Reason: "strict_session", Source: source}
	}
	if GetChannelConstraints(c).SuppressesRetry() {
		return PolicyDecision{Action: "stop", Reason: "pinned_channel", Source: "channel_constraint"}
	}
	if _, final := finalUpstreamErrorRule(err); final {
		return PolicyDecision{Action: "stop", Reason: "error_rule_no_retry", Source: "error_rule"}
	}
	if types.IsChannelError(err) {
		return PolicyDecision{Action: "retry", Reason: "channel_error", Source: "system"}
	}
	if types.IsSkipRetryError(err) {
		return PolicyDecision{Action: "stop", Reason: "non_retryable_error", Source: "system"}
	}
	if retryTimes <= 0 {
		return PolicyDecision{Action: "stop", Reason: "attempt_budget_exhausted", Source: "global"}
	}
	code := err.StatusCode
	if code >= 200 && code < 300 {
		return PolicyDecision{Action: "stop", Reason: "system_retry_exclusion", Source: "system"}
	}
	if code < 100 || code > 599 {
		return PolicyDecision{Action: "retry", Reason: "unrecognized_status", Source: "system"}
	}
	if operation_setting.IsAlwaysSkipRetryCode(err.GetErrorCode()) || operation_setting.IsAlwaysSkipRetryStatusCode(code) {
		return PolicyDecision{Action: "stop", Reason: "system_retry_exclusion", Source: "system"}
	}
	if operation_setting.ShouldRetryByStatusCode(code) {
		return PolicyDecision{Action: "retry", Reason: "retry_status_matched", Source: "global"}
	}
	return PolicyDecision{Action: "stop", Reason: "status_not_retryable", Source: "global"}
}

func ShouldRetryRelayError(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int) bool {
	return DecideRelayRetry(c, openaiErr, retryTimes).Action == "retry"
}

// UserFacingRelayError returns the error shown to API callers and in their
// usage logs. An upstream error that matches an upstream error rule is
// replaced: an intercept rule answers with the gateway's own "no available
// channel" error, so callers neither see upstream groups nor blame their own
// key or balance; a rewrite rule answers with the rule's own status code,
// error code and message. Server logs and administrator log views keep the
// original. The returned match is nil when the error was not replaced.
func UserFacingRelayError(c *gin.Context, err *types.NewAPIError) (*types.NewAPIError, *operation_setting.UpstreamErrorRuleMatch) {
	match, ok := matchUpstreamErrorRule(err)
	if !ok {
		return err, nil
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if response := match.Rule.Response; match.Rule.Action == operation_setting.UpstreamErrorActionRewrite && response != nil {
		message := strings.NewReplacer("{{model}}", modelName, "{{group}}", group).Replace(response.Message)
		return types.NewErrorWithStatusCode(errors.New(message), types.ErrorCode(response.Code), response.Status, types.ErrOptionWithPlainMessage()), &match
	}
	message := i18n.T(c, i18n.MsgDistributorNoAvailableChannel, map[string]any{"Group": group, "Model": modelName})
	return types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeModelNotFound, http.StatusServiceUnavailable), &match
}

// matchUpstreamErrorRule returns the first upstream error rule that matches an
// error the upstream returned, by its status code and its error code and
// message.
func matchUpstreamErrorRule(err *types.NewAPIError) (operation_setting.UpstreamErrorRuleMatch, bool) {
	if !types.IsUpstreamError(err) {
		return operation_setting.UpstreamErrorRuleMatch{}, false
	}
	return operation_setting.GetUpstreamErrorSetting().Match(err.StatusCode, string(err.GetErrorCode())+" "+err.Error())
}

// finalUpstreamErrorRule returns the upstream error rule that ends the request
// with this error. Like a request rule's error, it is not retried on this or
// another channel and does not count toward automatic disabling.
func finalUpstreamErrorRule(err *types.NewAPIError) (operation_setting.UpstreamErrorRuleMatch, bool) {
	match, ok := matchUpstreamErrorRule(err)
	if !ok || !match.Rule.SkipRetry {
		return operation_setting.UpstreamErrorRuleMatch{}, false
	}
	return match, true
}

func ProcessChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError, relayInfo *relaycommon.RelayInfo) {
	if err == nil {
		return
	}
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode())))
	if _, final := finalUpstreamErrorRule(err); !final && ShouldDisableChannel(err) && channelError.AutoBan {
		reason := err.MaskSensitiveErrorWithStatusCode()
		gopool.Go(func() {
			DisableChannel(channelError, reason)
		})
	}

	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		other := model.NewLogOther()
		if c.Request != nil && c.Request.URL != nil {
			other.SetPublic("request_path", c.Request.URL.Path)
		}
		// The log owner sees the same error as the API response; the stored
		// content and admin_info keep the upstream original for operators.
		visibleErr, rule := UserFacingRelayError(c, err)
		if rule != nil {
			other.SetUserContent(visibleErr.MaskSensitiveErrorWithStatusCode())
			other.SetAdmin("upstream_error", map[string]any{
				"error_type":  err.GetErrorType(),
				"error_code":  err.GetErrorCode(),
				"status_code": err.StatusCode,
				"rule":        rule.Position,
				"action":      rule.Rule.Action,
			})
		}
		other.SetPublic("error_type", visibleErr.GetErrorType())
		other.SetPublic("error_code", visibleErr.GetErrorCode())
		other.SetPublic("status_code", visibleErr.StatusCode)
		AppendRelayLogAdminInfo(c, relayInfo, other)
		if match, ok := common.GetContextKeyType[*requestrule.Match](c, constant.ContextKeyRequestRuleMatch); ok {
			other.SetAdmin("request_rule", map[string]any{
				"position": match.Index + 1,
				"id":       match.Rule.ID,
				"name":     match.Rule.Name,
				"field":    match.Field,
				"value":    match.Value,
			})
		}
		AppendResponseModelLogInfo(relayInfo, other)
		AppendTaskPluginContextAuditInfo(c, other)
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		model.RecordErrorLog(c, userId, channelError.ChannelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, other)
	}
}
