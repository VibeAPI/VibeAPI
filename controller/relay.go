package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			logger.LogError(c, fmt.Sprintf("relay error: %s", common.LocalLogPreview(newAPIError.Error())))
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
			case types.RelayFormatClaude:
				c.JSON(newAPIError.StatusCode, gin.H{
					"type":  "error",
					"error": newAPIError.ToClaudeError(),
				})
			default:
				c.JSON(newAPIError.StatusCode, gin.H{
					"error": newAPIError.ToOpenAIError(),
				})
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	initialChannel, auditErr := applyPromptAudit(c, relayInfo, request, nil)
	if auditErr != nil {
		newAPIError = auditErr
		return
	}
	if initialChannel != nil {
		if setupErr := middleware.SetupContextForSelectedChannel(c, initialChannel, relayInfo.OriginModelName); setupErr != nil {
			newAPIError = setupErr
			return
		}
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			newAPIError = types.NewError(err, types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}

	// common.SetContextKey(c, constant.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.OriginModelName))
	} else {
		newAPIError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if newAPIError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if newAPIError != nil {
			newAPIError = service.NormalizeViolationFeeError(newAPIError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			service.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			newAPIError = channelErr
			break
		}
		channel, channelErr = applyPromptAudit(c, relayInfo, request, channel)
		if channelErr != nil {
			newAPIError = channelErr
			break
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			relayInfo.LastError = nil
			return
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError

		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError)

		if !shouldRetry(c, newAPIError, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
	if newAPIError != nil {
		gopool.Go(func() {
			perfmetrics.RecordRelaySample(relayInfo, false, 0)
		})
	}
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	c.Set("use_channel", useChannel)
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if info.ChannelMeta == nil {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		return &model.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}, nil
	}
	if c.GetBool("prompt_audit_force_fallback") {
		setting := service.SnapshotPromptAuditSetting()
		selectGroup := common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
		if selectGroup == "" {
			selectGroup = retryParam.TokenGroup
		}
		channel, err := service.SelectPromptAuditFallback(selectGroup, retryParam.ModelName, retryParam.RequestPath, common.GetContextKeyInt(c, constant.ContextKeyUserId), c.GetInt64("prompt_audit_max_priority"), service.PromptAuditProtectedChannels(setting))
		if err != nil || channel == nil {
			if err == nil {
				err = errors.New("no unprotected channel is available")
			}
			return nil, types.NewErrorWithStatusCode(err, types.ErrorCodePromptAuditUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
		}
		if setupErr := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName); setupErr != nil {
			return nil, setupErr
		}
		return channel, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	if err != nil {
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %s", selectGroup, info.OriginModelName, err.Error()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

func applyPromptAudit(c *gin.Context, relayInfo *relaycommon.RelayInfo, request dto.Request, selected *model.Channel) (*model.Channel, *types.NewAPIError) {
	setting := service.SnapshotPromptAuditSetting()
	if !setting.Enabled || !service.ShouldPromptAuditUser(setting, relayInfo.UserId) {
		return selected, nil
	}
	channel := selected
	if channel == nil {
		var err error
		channel, err = model.CacheGetChannel(c.GetInt("channel_id"))
		if err != nil {
			return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeGetChannelFailed, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
		}
	}
	if !service.IsPromptAuditChannelProtected(setting, channel.Id) {
		return channel, nil
	}
	if c.GetBool("prompt_audit_checked") {
		return channel, nil
	}

	state, err := model.GetPromptAuditUserState(relayInfo.UserId)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodePromptAuditUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	content, truncated := service.ExtractPromptAuditContent(request, setting)
	if content == "" {
		c.Set("prompt_audit_checked", true)
		return channel, nil
	}
	restricted := service.PromptAuditStateRequiresFallback(setting, state, time.Now().Unix())
	if restricted && (state.PendingEventId != "" || setting.Mode == operation_setting.PromptAuditModeDowngrade) {
		return selectPromptAuditFallback(c, relayInfo, setting, channel)
	}

	fingerprint := model.PromptAuditFingerprint(relayInfo.UserId, content, setting.Version)
	recent, err := model.FindRecentPromptAuditEvent(relayInfo.UserId, fingerprint, setting.Version, time.Now().Add(-time.Duration(setting.DedupeMinutes)*time.Minute).Unix())
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodePromptAuditUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	if recent != nil {
		if recent.Status == model.PromptAuditEventPending {
			if setting.Mode == operation_setting.PromptAuditModeDowngrade {
				return selectPromptAuditFallback(c, relayInfo, setting, channel)
			}
			waitCtx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(setting.ReviewTotalTimeoutSeconds)*time.Second)
			defer cancel()
			recent, err = service.WaitPromptAuditEvent(waitCtx, recent.EventId)
			if err != nil {
				return nil, types.NewErrorWithStatusCode(errors.New("prompt audit review timed out"), types.ErrorCodePromptAuditUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
			}
		}
		if recent.Status == model.PromptAuditEventViolation {
			if setting.Mode == operation_setting.PromptAuditModeReject {
				return nil, promptAuditRejectedError(setting, recent.EventId)
			}
			return selectPromptAuditFallback(c, relayInfo, setting, channel)
		}
		if recent.Status == model.PromptAuditEventFailed {
			return selectPromptAuditFallback(c, relayInfo, setting, channel)
		}
		if restricted {
			return selectPromptAuditFallback(c, relayInfo, setting, channel)
		}
		c.Set("prompt_audit_checked", true)
		return channel, nil
	}

	decision, duration, err := service.RunPromptAuditMain(c.Request.Context(), setting, content)
	if err != nil {
		if eventID := service.MarkPromptAuditTechnicalFailure(c, setting, content, truncated, channel.Id, duration, err); eventID != "" {
			c.Set("prompt_audit_event_id", eventID)
		}
		return selectPromptAuditFallback(c, relayInfo, setting, channel)
	}
	if !decision.Flagged || decision.Confidence < setting.MainThreshold {
		eventID, recordErr := service.RecordPromptAuditMainSafe(c, setting, content, truncated, decision, channel.Id, duration)
		if recordErr != nil {
			return selectPromptAuditFallback(c, relayInfo, setting, channel)
		}
		c.Set("prompt_audit_event_id", eventID)
		if restricted {
			return selectPromptAuditFallback(c, relayInfo, setting, channel)
		}
		c.Set("prompt_audit_checked", true)
		return channel, nil
	}
	if !setting.ReviewEnabled {
		eventID, recordErr := service.RecordPromptAuditMainViolation(c, setting, content, truncated, decision, channel.Id, duration)
		if recordErr != nil {
			return selectPromptAuditFallback(c, relayInfo, setting, channel)
		}
		c.Set("prompt_audit_event_id", eventID)
		if setting.Mode == operation_setting.PromptAuditModeReject {
			return nil, promptAuditRejectedError(setting, eventID)
		}
		return selectPromptAuditFallback(c, relayInfo, setting, channel)
	}
	event, err := service.CreatePromptAuditEvent(c, setting, content, truncated, decision, channel.Id, duration)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodePromptAuditUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	c.Set("prompt_audit_event_id", event.EventId)
	outcome := service.PromptAuditOutcome{EventId: event.EventId, Decision: decision, Event: event, Setting: setting, Content: content}
	if setting.Mode == operation_setting.PromptAuditModeDowngrade {
		go func() {
			if _, reviewErr := service.FinishPromptAuditReviews(context.Background(), outcome); reviewErr != nil {
				common.SysError("prompt audit review failed: " + reviewErr.Error())
			}
		}()
		return selectPromptAuditFallback(c, relayInfo, setting, channel)
	}
	confirmed, err := service.FinishPromptAuditReviews(c.Request.Context(), outcome)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodePromptAuditUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	if confirmed {
		return nil, promptAuditRejectedError(setting, event.EventId)
	}
	finishedEvent, finishErr := model.GetPromptAuditEvent(event.EventId)
	if finishErr != nil {
		return nil, types.NewErrorWithStatusCode(finishErr, types.ErrorCodePromptAuditUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	if finishedEvent.Status == model.PromptAuditEventFailed {
		return selectPromptAuditFallback(c, relayInfo, setting, channel)
	}
	if restricted {
		return selectPromptAuditFallback(c, relayInfo, setting, channel)
	}
	c.Set("prompt_audit_checked", true)
	return channel, nil
}

func selectPromptAuditFallback(c *gin.Context, relayInfo *relaycommon.RelayInfo, setting operation_setting.PromptAuditSetting, origin *model.Channel) (*model.Channel, *types.NewAPIError) {
	selectGroup := common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	if selectGroup == "" {
		selectGroup = relayInfo.TokenGroup
	}
	channel, err := service.SelectPromptAuditFallback(selectGroup, relayInfo.OriginModelName, c.Request.URL.Path, relayInfo.UserId, origin.GetPriority(), service.PromptAuditProtectedChannels(setting))
	if err != nil || channel == nil {
		if err == nil {
			err = errors.New("no unprotected channel is available")
		}
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodePromptAuditUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	c.Set("prompt_audit_checked", true)
	c.Set("prompt_audit_force_fallback", true)
	c.Set("prompt_audit_max_priority", origin.GetPriority())
	if eventID := c.GetString("prompt_audit_event_id"); eventID != "" {
		_ = model.UpdatePromptAuditEventFinalChannel(eventID, channel.Id)
	}
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
		return nil, setupErr
	}
	return channel, nil
}

func promptAuditRejectedError(setting operation_setting.PromptAuditSetting, eventId string) *types.NewAPIError {
	message := strings.TrimSpace(setting.RejectMessage)
	if message == "" {
		message = "Your request was blocked by the content policy."
	}
	return types.NewErrorWithStatusCode(fmt.Errorf("%s Event ID: %s", message, eventId), types.ErrorCodeContentPolicyViolation, http.StatusForbidden, types.ErrOptionWithSkipRetry())
}

func shouldRetry(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if types.IsChannelError(openaiErr) {
		return true
	}
	if types.IsSkipRetryError(openaiErr) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false
	}
	return operation_setting.ShouldRetryByStatusCode(code)
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError) {
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, common.LocalLogPreview(err.Error())))
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously
	if service.ShouldDisableChannel(err) && channelError.AutoBan {
		gopool.Go(func() {
			service.DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}

	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelId := c.GetInt("channel_id")
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["channel_id"] = channelId
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")
		adminInfo := make(map[string]interface{})
		adminInfo["use_channel"] = c.GetStringSlice("use_channel")
		isMultiKey := common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
		if isMultiKey {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		service.AppendChannelAffinityAdminInfo(c, adminInfo)
		other["admin_info"] = adminInfo
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		model.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, other)
	}

}

func RelayMidjourney(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *dto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func RelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var auditTaskRequest relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &auditTaskRequest); err != nil {
		respondTaskError(c, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest))
		return
	}
	if strings.TrimSpace(auditTaskRequest.Prompt) != "" {
		auditRequest := &dto.ImageRequest{Prompt: auditTaskRequest.Prompt, Model: relayInfo.OriginModelName}
		initialChannel, auditErr := applyPromptAudit(c, relayInfo, auditRequest, nil)
		if auditErr != nil {
			respondTaskError(c, service.TaskErrorWrapperLocal(auditErr.Err, string(auditErr.GetErrorCode()), auditErr.StatusCode))
			return
		}
		if initialChannel != nil {
			if setupErr := middleware.SetupContextForSelectedChannel(c, initialChannel, relayInfo.OriginModelName); setupErr != nil {
				respondTaskError(c, service.TaskErrorWrapperLocal(setupErr.Err, "setup_channel_failed", setupErr.StatusCode))
				return
			}
		}
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil && !c.GetBool("prompt_audit_force_fallback") {
			channel = lockedCh
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
		}
		channel, channelErr := applyPromptAudit(c, relayInfo, &dto.ImageRequest{Prompt: auditTaskRequest.Prompt, Model: relayInfo.OriginModelName}, channel)
		if channelErr != nil {
			taskErr = service.TaskErrorWrapperLocal(channelErr.Err, string(channelErr.GetErrorCode()), channelErr.StatusCode)
			break
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}

		if !taskErr.LocalError {
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
		}

		if !shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	// ── 成功：结算 + 日志 + 插入任务 ──
	if taskErr == nil {
		if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError("settle task billing error: " + settleErr.Error())
		}
		service.LogTaskConsumption(c, relayInfo)

		task := model.InitTask(result.Platform, relayInfo)
		task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
		if relayInfo.ChannelMeta != nil && relayInfo.ChannelMeta.ChannelIsMultiKey {
			multiKeyIndex := relayInfo.ChannelMeta.ChannelMultiKeyIndex
			task.PrivateData.ChannelMultiKeyIndex = &multiKeyIndex
			task.PrivateData.ChannelKeyFingerprint = fmt.Sprintf("%x", common.Sha256Raw([]byte(relayInfo.ChannelMeta.ApiKey)))
		}
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.NodeName = common.NodeName
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      relayInfo.PriceData.ModelPrice,
			GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      relayInfo.PriceData.ModelRatio,
			OtherRatios:     relayInfo.PriceData.OtherRatios(),
			OriginModelName: relayInfo.OriginModelName,
			PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
		}
		task.Quota = result.Quota
		task.Data = result.TaskData
		task.Action = relayInfo.Action
		if insertErr := task.Insert(); insertErr != nil {
			common.SysError("insert task error: " + insertErr.Error())
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if taskErr.StatusCode == 307 {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest {
		return false
	}
	if taskErr.StatusCode == 408 {
		// azure处理超时不重试
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}
