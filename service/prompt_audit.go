package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	promptAuditVoteCount       = 5
	promptAuditMaxResponseBody = 64 << 10
	promptAuditMaxReasonRunes  = 1000
)

var promptAuditCredentialPattern = regexp.MustCompile(`(?i)(bearer\s+|api[-_ ]?key\s*[:=]\s*|password\s*[:=]\s*)([A-Za-z0-9_./+\-=]{8,})`)

type PromptAuditDecision struct {
	Flagged    bool     `json:"flagged"`
	Confidence float64  `json:"confidence"`
	Categories []string `json:"categories"`
	Reason     string   `json:"reason"`
}

type PromptAuditVote struct {
	Valid      bool     `json:"valid"`
	Flagged    bool     `json:"flagged"`
	Confidence float64  `json:"confidence,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type PromptAuditOutcome struct {
	EventId  string
	Decision PromptAuditDecision
	Event    *model.PromptAuditEvent
	Setting  operation_setting.PromptAuditSetting
	Content  string
}

type promptAuditChatRequest struct {
	Model       string                   `json:"model"`
	Messages    []promptAuditChatMessage `json:"messages"`
	Temperature float64                  `json:"temperature"`
	MaxTokens   int                      `json:"max_tokens"`
}

type promptAuditChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type promptAuditChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func SnapshotPromptAuditSetting() operation_setting.PromptAuditSetting {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	current := operation_setting.GetPromptAuditSetting()
	snapshot := *current
	snapshot.ProtectedChannelIds = append([]int(nil), current.ProtectedChannelIds...)
	snapshot.AudienceUserIds = append([]int(nil), current.AudienceUserIds...)
	return snapshot
}

func PromptAuditProtectedChannels(setting operation_setting.PromptAuditSetting) map[int]struct{} {
	protected := make(map[int]struct{}, len(setting.ProtectedChannelIds))
	for _, channelId := range setting.ProtectedChannelIds {
		if channelId > 0 {
			protected[channelId] = struct{}{}
		}
	}
	return protected
}

func ShouldPromptAuditUser(setting operation_setting.PromptAuditSetting, userId int) bool {
	listed := false
	for _, candidate := range setting.AudienceUserIds {
		if candidate == userId {
			listed = true
			break
		}
	}
	switch setting.AudienceMode {
	case operation_setting.PromptAuditScopeWhitelist:
		return !listed
	case operation_setting.PromptAuditScopeBlacklist:
		return listed
	default:
		return true
	}
}

func IsPromptAuditChannelProtected(setting operation_setting.PromptAuditSetting, channelId int) bool {
	_, ok := PromptAuditProtectedChannels(setting)[channelId]
	return ok
}

func PromptAuditStateRequiresFallback(setting operation_setting.PromptAuditSetting, state *model.PromptAuditUserState, now int64) bool {
	if !setting.Enabled || state == nil {
		return false
	}
	if state.PendingEventId != "" || state.IndefinitelyBlocked {
		return true
	}
	return state.RestrictedUntil > now
}

func ExtractPromptAuditContent(request dto.Request, setting operation_setting.PromptAuditSetting) (string, bool) {
	if request == nil {
		return "", false
	}
	var text string
	if setting.ContentScope == operation_setting.PromptAuditContentAll {
		if meta := request.GetTokenCountMeta(); meta != nil {
			text = meta.CombineText
		}
	} else {
		text = latestPromptAuditContent(request, setting.ContentScope == operation_setting.PromptAuditContentLatestTools)
		if text == "" {
			if meta := request.GetTokenCountMeta(); meta != nil {
				text = meta.CombineText
			}
		}
	}
	text = promptAuditCredentialPattern.ReplaceAllString(text, "${1}[REDACTED]")
	maxCharacters := setting.MaxCharacters
	if maxCharacters <= 0 {
		maxCharacters = 40000
	}
	if utf8.RuneCountInString(text) <= maxCharacters {
		return strings.TrimSpace(text), false
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[len(runes)-maxCharacters:])), true
}

func latestPromptAuditContent(request dto.Request, includeTools bool) string {
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		latestUser := -1
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				latestUser = i
				break
			}
		}
		if latestUser < 0 {
			return ""
		}
		parts := make([]string, 0, 2)
		for _, item := range req.Messages[latestUser].ParseContent() {
			if item.Type == dto.ContentTypeText && item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
		if includeTools {
			for i := latestUser + 1; i < len(req.Messages); i++ {
				if req.Messages[i].Role != "tool" {
					continue
				}
				for _, item := range req.Messages[i].ParseContent() {
					if item.Type == dto.ContentTypeText && item.Text != "" {
						parts = append(parts, item.Text)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	case *dto.ClaudeRequest:
		parts := make([]string, 0, 2)
		for i := len(req.Messages) - 1; i >= 0; i-- {
			message := req.Messages[i]
			if message.Role == "user" {
				if text := message.GetStringContent(); text != "" {
					parts = append(parts, text)
				}
				if includeTools {
					content, _ := message.ParseContent()
					for _, item := range content {
						if item.Type == "tool_result" && item.Content != nil {
							encoded, _ := common.Marshal(item.Content)
							parts = append(parts, string(encoded))
						}
					}
				}
				break
			}
		}
		return strings.Join(parts, "\n")
	case *dto.OpenAIResponsesRequest:
		inputs := req.ParseInput()
		parts := make([]string, 0, len(inputs))
		for _, input := range inputs {
			if input.Text != "" {
				parts = append(parts, input.Text)
			}
		}
		return strings.Join(parts, "\n")
	case *dto.GeminiChatRequest:
		for i := len(req.Contents) - 1; i >= 0; i-- {
			content := req.Contents[i]
			if content.Role != "user" && content.Role != "" {
				continue
			}
			parts := make([]string, 0, len(content.Parts))
			for _, part := range content.Parts {
				if part.Text != "" {
					parts = append(parts, part.Text)
				}
			}
			return strings.Join(parts, "\n")
		}
	default:
		if meta := request.GetTokenCountMeta(); meta != nil {
			return meta.CombineText
		}
	}
	return ""
}

func RunPromptAuditMain(ctx context.Context, setting operation_setting.PromptAuditSetting, content string) (PromptAuditDecision, time.Duration, error) {
	return callPromptAuditEndpoint(ctx, setting, setting.Main, content, nil)
}

func RunPromptAuditReview(ctx context.Context, setting operation_setting.PromptAuditSetting, content string, first PromptAuditDecision) (PromptAuditDecision, time.Duration, error) {
	return callPromptAuditEndpoint(ctx, setting, effectiveReviewEndpoint(setting), content, &first)
}

func callPromptAuditEndpoint(ctx context.Context, setting operation_setting.PromptAuditSetting, endpoint operation_setting.PromptAuditEndpoint, content string, first *PromptAuditDecision) (PromptAuditDecision, time.Duration, error) {
	started := time.Now()
	apiKey, err := common.DecryptSecret(endpoint.APIKeyEncrypted)
	if err != nil {
		return PromptAuditDecision{}, 0, fmt.Errorf("decrypt audit API key: %w", err)
	}
	if endpoint.URL == "" || endpoint.Model == "" || apiKey == "" {
		return PromptAuditDecision{}, 0, errors.New("audit endpoint is incomplete")
	}
	userContent := "<user_input>\n" + content + "\n</user_input>"
	if first != nil {
		encoded, _ := common.Marshal(first)
		userContent += "\n<first_stage_result>" + string(encoded) + "</first_stage_result>"
	}
	userContent += `\nReturn exactly one JSON object with this schema: {"flagged":boolean,"confidence":number,"categories":string[],"reason":string}.`
	payload, err := common.Marshal(promptAuditChatRequest{
		Model: endpoint.Model, Temperature: 0, MaxTokens: 512,
		Messages: []promptAuditChatMessage{{Role: "system", Content: endpoint.SystemPrompt}, {Role: "user", Content: userContent}},
	})
	if err != nil {
		return PromptAuditDecision{}, 0, err
	}
	timeout := time.Duration(endpoint.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint.URL, bytes.NewReader(payload))
	if err != nil {
		return PromptAuditDecision{}, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := promptAuditHTTPClient(setting.AllowPrivateEndpoints, timeout)
	resp, err := client.Do(req)
	if err != nil {
		return PromptAuditDecision{}, time.Since(started), err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, promptAuditMaxResponseBody+1))
	if err != nil {
		return PromptAuditDecision{}, time.Since(started), err
	}
	if len(body) > promptAuditMaxResponseBody {
		return PromptAuditDecision{}, time.Since(started), errors.New("audit response is too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PromptAuditDecision{}, time.Since(started), fmt.Errorf("audit endpoint returned HTTP %d", resp.StatusCode)
	}
	var chatResponse promptAuditChatResponse
	if err := common.Unmarshal(body, &chatResponse); err != nil || len(chatResponse.Choices) == 0 {
		return PromptAuditDecision{}, time.Since(started), errors.New("invalid audit chat response")
	}
	raw := strings.TrimSpace(chatResponse.Choices[0].Message.Content)
	if strings.HasPrefix(raw, "```") {
		return PromptAuditDecision{}, time.Since(started), errors.New("audit model did not return strict JSON")
	}
	var decision PromptAuditDecision
	var rawObject map[string]interface{}
	if err := common.UnmarshalJsonStr(raw, &rawObject); err != nil {
		return PromptAuditDecision{}, time.Since(started), errors.New("audit response must be a JSON object")
	}
	allowedKeys := map[string]struct{}{"flagged": {}, "confidence": {}, "categories": {}, "reason": {}}
	if len(rawObject) != len(allowedKeys) {
		return PromptAuditDecision{}, time.Since(started), errors.New("audit response must contain exactly flagged, confidence, categories, and reason")
	}
	for key := range rawObject {
		if _, ok := allowedKeys[key]; !ok {
			return PromptAuditDecision{}, time.Since(started), fmt.Errorf("audit response contains unknown field %q", key)
		}
	}
	if err := common.UnmarshalJsonStr(raw, &decision); err != nil {
		return PromptAuditDecision{}, time.Since(started), fmt.Errorf("invalid audit decision: %w", err)
	}
	if decision.Confidence < 0 || decision.Confidence > 1 || len([]rune(decision.Reason)) > promptAuditMaxReasonRunes {
		return PromptAuditDecision{}, time.Since(started), errors.New("audit decision violates schema bounds")
	}
	if decision.Categories == nil {
		decision.Categories = []string{}
	}
	return decision, time.Since(started), nil
}

func promptAuditHTTPClient(allowPrivate bool, timeout time.Duration) *http.Client {
	netDialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		MaxIdleConns: common.RelayMaxIdleConns, MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		IdleConnTimeout: time.Duration(common.RelayIdleConnTimeout) * time.Second, ForceAttemptHTTP2: true,
		DialContext: promptAuditDialer{allowPrivate: allowPrivate, dialContext: netDialer.DialContext}.DialContext,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if err := validatePromptAuditRuntimeURL(req.URL, allowPrivate); err != nil {
			return err
		}
		if len(via) >= 5 {
			return errors.New("too many audit endpoint redirects")
		}
		return nil
	}}
}

type promptAuditDialer struct {
	allowPrivate bool
	dialContext  func(ctx context.Context, network, address string) (net.Conn, error)
}

func (d promptAuditDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var candidates []net.IP
	for _, address := range resolved {
		if err := validatePromptAuditIP(address.IP, d.allowPrivate); err != nil {
			return nil, fmt.Errorf("audit endpoint %s: %w", host, err)
		}
		candidates = append(candidates, address.IP)
	}
	var lastErr error
	for _, ip := range candidates {
		connection, err := d.dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("audit endpoint resolved to no usable addresses")
}

func validatePromptAuditRuntimeURL(target *url.URL, allowPrivate bool) error {
	if target == nil || target.Scheme != "https" || target.Hostname() == "" {
		return errors.New("audit endpoint must be HTTPS")
	}
	host := target.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		return validatePromptAuditIP(ip, allowPrivate)
	}
	resolved, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range resolved {
		if err := validatePromptAuditIP(ip, allowPrivate); err != nil {
			return fmt.Errorf("audit endpoint %s: %w", host, err)
		}
	}
	return nil
}

func validatePromptAuditIP(ip net.IP, allowPrivate bool) error {
	if ip == nil {
		return errors.New("resolved to an invalid address")
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.Equal(net.ParseIP("100.100.100.200")) {
		return errors.New("metadata and link-local addresses are blocked")
	}
	if allowPrivate {
		return nil
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() || promptAuditCGNAT.Contains(ip) {
		return errors.New("private or special-use addresses require explicit opt-in")
	}
	return nil
}

var _, promptAuditCGNAT, _ = net.ParseCIDR("100.64.0.0/10")

func ValidatePromptAuditEndpoint(rawURL string, allowPrivate bool) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("audit endpoint must be a complete HTTPS URL")
	}
	return validatePromptAuditRuntimeURL(parsed, allowPrivate)
}

func CreatePromptAuditEvent(c *gin.Context, setting operation_setting.PromptAuditSetting, content string, truncated bool, decision PromptAuditDecision, originChannelId int, duration time.Duration) (*model.PromptAuditEvent, error) {
	event := &model.PromptAuditEvent{
		EventId: common.NewRequestId(), UserId: c.GetInt("id"), RequestId: c.GetString(common.RequestIdKey),
		ContentFingerprint: model.PromptAuditFingerprint(c.GetInt("id"), content, setting.Version), ConfigVersion: setting.Version,
		Status: model.PromptAuditEventPending, MainConfidence: decision.Confidence, OriginChannelId: originChannelId,
		ContentTruncated: truncated, LatencyMs: duration.Milliseconds(), EmailStatus: "not_required",
	}
	encodedCategories, _ := common.Marshal(decision.Categories)
	event.Categories = string(encodedCategories)
	if err := model.CreatePromptAuditEventAndMarkPending(event); err != nil {
		return nil, err
	}
	return event, nil
}

func RecordPromptAuditMainSafe(c *gin.Context, setting operation_setting.PromptAuditSetting, content string, truncated bool, decision PromptAuditDecision, channelId int, duration time.Duration) (string, error) {
	event := &model.PromptAuditEvent{
		EventId: common.NewRequestId(), UserId: c.GetInt("id"), RequestId: c.GetString(common.RequestIdKey),
		ContentFingerprint: model.PromptAuditFingerprint(c.GetInt("id"), content, setting.Version), ConfigVersion: setting.Version,
		Status: model.PromptAuditEventSafe, MainConfidence: decision.Confidence, OriginChannelId: channelId,
		FinalChannelId: channelId, ContentTruncated: truncated, LatencyMs: duration.Milliseconds(), EmailStatus: "not_required",
	}
	encodedCategories, _ := common.Marshal(decision.Categories)
	event.Categories = string(encodedCategories)
	if err := model.DB.Create(event).Error; err != nil {
		return "", err
	}
	return event.EventId, nil
}

func RecordPromptAuditMainViolation(c *gin.Context, setting operation_setting.PromptAuditSetting, content string, truncated bool, decision PromptAuditDecision, channelId int, duration time.Duration) (string, error) {
	event, err := CreatePromptAuditEvent(c, setting, content, truncated, decision, channelId, duration)
	if err != nil {
		return "", err
	}
	categories, _ := common.Marshal(decision.Categories)
	restrictionHours := setting.FirstRestrictionHours
	state, _ := model.GetPromptAuditUserState(event.UserId)
	if state != nil && state.ViolationCount >= 1 {
		restrictionHours = setting.SecondRestrictionHours
	}
	if err := model.FinishPromptAuditEvent(event.EventId, model.PromptAuditEventViolation, string(categories), 1, 1, "[]", "", duration.Milliseconds(), restrictionHours, setting.ViolationResetDays); err != nil {
		return "", err
	}
	go SendPromptAuditNotification(event.EventId, setting)
	return event.EventId, nil
}

func FinishPromptAuditReviews(ctx context.Context, outcome PromptAuditOutcome) (bool, error) {
	setting := outcome.Setting
	totalTimeout := time.Duration(setting.ReviewTotalTimeoutSeconds) * time.Second
	if totalTimeout <= 0 {
		totalTimeout = 15 * time.Second
	}
	reviewCtx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()
	votes := make([]PromptAuditVote, promptAuditVoteCount)
	var wg sync.WaitGroup
	for i := range votes {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			decision, _, err := callPromptAuditEndpoint(reviewCtx, setting, effectiveReviewEndpoint(setting), outcome.Content, &outcome.Decision)
			if err != nil && reviewCtx.Err() == nil {
				decision, _, err = callPromptAuditEndpoint(reviewCtx, setting, effectiveReviewEndpoint(setting), outcome.Content, &outcome.Decision)
			}
			if err != nil {
				votes[index] = PromptAuditVote{Error: safeAuditError(err)}
				return
			}
			votes[index] = PromptAuditVote{Valid: true, Flagged: decision.Flagged && decision.Confidence >= setting.ReviewThreshold, Confidence: decision.Confidence, Categories: decision.Categories}
		}(i)
	}
	wg.Wait()
	validVotes := 0
	flaggedVotes := 0
	categories := make(map[string]struct{})
	for _, vote := range votes {
		if !vote.Valid {
			continue
		}
		validVotes++
		if vote.Flagged {
			flaggedVotes++
		}
		for _, category := range vote.Categories {
			categories[category] = struct{}{}
		}
	}
	confirmed := validVotes >= setting.RequiredValidVotes && flaggedVotes >= setting.RequiredFlaggedVotes
	// Disabling audit or changing its configuration invalidates in-flight reviews;
	// late results must never punish a user.
	current := SnapshotPromptAuditSetting()
	if !current.Enabled || current.Version != setting.Version {
		confirmed = false
		validVotes = 0
		flaggedVotes = 0
	}
	status := model.PromptAuditEventSafe
	failureType := ""
	if confirmed {
		status = model.PromptAuditEventViolation
	} else if validVotes < setting.RequiredValidVotes {
		status = model.PromptAuditEventFailed
		failureType = "insufficient_valid_votes"
	}
	categoryList := make([]string, 0, len(categories))
	for category := range categories {
		categoryList = append(categoryList, category)
	}
	sort.Strings(categoryList)
	categoryJSON, _ := common.Marshal(categoryList)
	votesJSON, _ := common.Marshal(votes)
	restrictionHours := setting.FirstRestrictionHours
	state, _ := model.GetPromptAuditUserState(outcome.Event.UserId)
	if state != nil && state.ViolationCount >= 1 {
		restrictionHours = setting.SecondRestrictionHours
	}
	if err := model.FinishPromptAuditEvent(outcome.EventId, status, string(categoryJSON), validVotes, flaggedVotes, string(votesJSON), failureType, time.Since(time.Unix(outcome.Event.CreatedAt, 0)).Milliseconds(), restrictionHours, setting.ViolationResetDays); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if confirmed {
		go SendPromptAuditNotification(outcome.EventId, setting)
	}
	return confirmed, nil
}

func effectiveReviewEndpoint(setting operation_setting.PromptAuditSetting) operation_setting.PromptAuditEndpoint {
	endpoint := setting.Review
	if endpoint.URL == "" {
		endpoint.URL = setting.Main.URL
	}
	if endpoint.APIKeyEncrypted == "" {
		endpoint.APIKeyEncrypted = setting.Main.APIKeyEncrypted
	}
	return endpoint
}

func safeAuditError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 160 {
		message = message[:160]
	}
	return message
}

func SendPromptAuditNotification(eventId string, setting operation_setting.PromptAuditSetting) {
	event, err := model.GetPromptAuditEvent(eventId)
	if err != nil || event.EmailStatus == "sent" {
		return
	}
	email, err := model.GetUserEmail(event.UserId)
	if err != nil || strings.TrimSpace(email) == "" {
		_ = model.UpdatePromptAuditEventEmail(eventId, "failed", 0, "user has no email address")
		return
	}
	state, _ := model.GetPromptAuditUserState(event.UserId)
	restriction := "until administrator review"
	if state != nil && !state.IndefinitelyBlocked && state.RestrictedUntil > 0 {
		restriction = time.Unix(state.RestrictedUntil, 0).Format(time.RFC3339)
	}
	language := model.GetUserLanguage(event.UserId)
	subject := "Content policy notice"
	content := fmt.Sprintf("<p>Your request triggered the service content policy.</p><p>Event: %s</p><p>Time: %s</p><p>Categories: %s</p><p>Protected-channel restriction: %s</p>", html.EscapeString(event.EventId), time.Unix(event.CreatedAt, 0).Format(time.RFC3339), html.EscapeString(event.Categories), html.EscapeString(restriction))
	if strings.HasPrefix(strings.ToLower(language), "zh") {
		subject = "内容策略提醒"
		content = fmt.Sprintf("<p>你的请求触发了服务内容策略。</p><p>事件：%s</p><p>时间：%s</p><p>类别：%s</p><p>受保护渠道限制：%s</p>", html.EscapeString(event.EventId), time.Unix(event.CreatedAt, 0).Format(time.RFC3339), html.EscapeString(event.Categories), html.EscapeString(restriction))
	}
	if setting.AppealContact != "" {
		label := "Appeal contact: "
		if strings.HasPrefix(strings.ToLower(language), "zh") {
			label = "申诉联系方式："
		}
		content += "<p>" + label + html.EscapeString(setting.AppealContact) + "</p>"
	}
	var sendErr error
	for attempt := 1; attempt <= 3; attempt++ {
		sendErr = common.SendEmail(subject, email, content)
		if sendErr == nil {
			_ = model.UpdatePromptAuditEventEmail(eventId, "sent", attempt, "")
			return
		}
	}
	_ = model.UpdatePromptAuditEventEmail(eventId, "failed", 3, safeAuditError(sendErr))
}

func MarkPromptAuditTechnicalFailure(c *gin.Context, setting operation_setting.PromptAuditSetting, content string, truncated bool, originChannelId int, duration time.Duration, failure error) string {
	event := &model.PromptAuditEvent{
		EventId: common.NewRequestId(), UserId: c.GetInt("id"), RequestId: c.GetString(common.RequestIdKey),
		ContentFingerprint: model.PromptAuditFingerprint(c.GetInt("id"), content, setting.Version), ConfigVersion: setting.Version,
		Status: model.PromptAuditEventFailed, OriginChannelId: originChannelId, ContentTruncated: truncated,
		FailureType: safeAuditError(failure), LatencyMs: duration.Milliseconds(), EmailStatus: "not_required",
	}
	if err := model.DB.Create(event).Error; err != nil {
		logger.LogError(c, "failed to save prompt audit failure: "+err.Error())
		return ""
	}
	return event.EventId
}

func SelectPromptAuditFallback(group string, modelName string, requestPath string, userId int, maxPriority int64, protected map[int]struct{}) (*model.Channel, error) {
	if group == "" || group == "auto" {
		return nil, errors.New("prompt audit fallback requires the selected concrete group")
	}
	if common.MemoryCacheEnabled {
		return model.GetRandomUnprotectedChannel(group, modelName, requestPath, userId, maxPriority, protected)
	}
	return model.GetUnprotectedChannel(group, modelName, requestPath, userId, maxPriority, protected)
}

func WaitPromptAuditEvent(ctx context.Context, eventId string) (*model.PromptAuditEvent, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		event, err := model.GetPromptAuditEvent(eventId)
		if err != nil {
			return nil, err
		}
		if event.Status != model.PromptAuditEventPending {
			return event, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// StartPromptAuditMaintenanceTask performs bounded retention cleanup on the
// master node. Pending events older than one review window are failed so a
// process restart cannot leave user state permanently blocked.
func StartPromptAuditMaintenanceTask() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		run := func() {
			setting := SnapshotPromptAuditSetting()
			staleAfter := time.Duration(setting.ReviewTotalTimeoutSeconds+30) * time.Second
			if staleAfter < time.Minute {
				staleAfter = time.Minute
			}
			if err := model.FailStalePromptAuditEvents(time.Now().Add(-staleAfter).Unix()); err != nil {
				common.SysError("prompt audit stale-event recovery failed: " + err.Error())
			}
			if err := model.CleanupPromptAuditEvents(setting.RetentionDays); err != nil {
				common.SysError("prompt audit retention cleanup failed: " + err.Error())
			}
		}
		run()
		for range ticker.C {
			run()
		}
	}()
}
