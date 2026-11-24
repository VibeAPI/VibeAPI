package operation_setting

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// Actions decide the error a caller sees when an upstream error matches a
// rule. Both only change what callers see: retries and automatic disabling
// still follow the upstream's original error.
const (
	// UpstreamErrorActionIntercept answers with the gateway's own "no
	// available channel" error (503), so callers neither see the upstream's
	// account problem nor blame their own key or balance.
	UpstreamErrorActionIntercept = "intercept"
	// UpstreamErrorActionRewrite answers with the rule's own status code,
	// error code and message.
	UpstreamErrorActionRewrite = "rewrite"
)

const (
	maxUpstreamErrorRules        = 100
	maxUpstreamErrorKeywords     = 200
	maxUpstreamErrorKeywordRunes = 200
	maxUpstreamErrorNameRunes    = 60
	maxUpstreamErrorMessageRunes = 500
)

var upstreamErrorCodePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// UpstreamErrorSetting decides what callers see when an upstream channel
// returns an error.
type UpstreamErrorSetting struct {
	// Rules is a JSON array of UpstreamErrorRule, checked in order; the first
	// enabled rule that matches decides the caller's error. An empty value
	// means the rules were never saved, and the legacy fields below apply.
	Rules string `json:"rules"`
	// The legacy fields hid upstream account errors before rules existed.
	// Until rules are saved they read as one intercept rule for the status
	// codes and one for the keywords, so saved settings keep working.
	MaskEnabled     bool   `json:"mask_enabled"`
	MaskStatusCodes string `json:"mask_status_codes"`
	MaskKeywords    string `json:"mask_keywords"`
}

// UpstreamErrorRule matches upstream errors by status code and by keywords in
// the error code or message. A rule needs at least one of the two; when it
// has both, both must match.
type UpstreamErrorRule struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	// StatusCodes holds comma-separated status codes and inclusive ranges.
	// Empty matches any status.
	StatusCodes string `json:"status_codes"`
	// Keywords match the upstream error code or message, ignoring case; any
	// one keyword is enough. Empty matches any message.
	Keywords []string `json:"keywords"`
	Action   string   `json:"action"`
	// Response is the caller's error for rewrite rules and is ignored for
	// intercept rules.
	Response *UpstreamErrorResponse `json:"response,omitempty"`
	// SkipRetry ends the request with this rule's error at once, like a
	// request rule: it is not retried on this or another channel and does not
	// count toward automatic disabling. Otherwise the global retry and
	// automatic disable settings apply to the original upstream error.
	SkipRetry bool `json:"skip_retry,omitempty"`
}

// UpstreamErrorResponse is the error a rewrite rule answers with. Message may
// use {{model}} and {{group}}; the request ID is appended when it is sent.
type UpstreamErrorResponse struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// UpstreamErrorRuleMatch is the first rule an upstream error matched.
type UpstreamErrorRuleMatch struct {
	// Position is the rule's place in the list, starting at 1.
	Position int
	Rule     UpstreamErrorRule
}

var upstreamErrorSetting = UpstreamErrorSetting{
	MaskEnabled:     true,
	MaskStatusCodes: "401-403",
	MaskKeywords: strings.Join([]string{
		"insufficient_quota",
		"insufficient quota",
		"insufficient_user_quota",
		"pre_consume_token_quota_failed",
		"exceeded your current quota",
		"credit balance is too low",
		"insufficient balance",
		"insufficient_balance",
		"insufficient credits",
		"billing_not_active",
		"arrearage",
		"额度不足",
		"额度已用尽",
		"余额不足",
		"欠费",
		"model_not_found",
		"model not found",
		"model not exist",
		"no available channel",
		"has no access to model",
		"可用渠道",
		"模型不存在",
		"无权访问模型",
	}, "\n"),
}

func init() {
	config.GlobalConfig.Register("upstream_error_setting", &upstreamErrorSetting)
}

func GetUpstreamErrorSetting() *UpstreamErrorSetting {
	return &upstreamErrorSetting
}

// ParseUpstreamErrorRules validates a JSON array of upstream error rules; an
// empty string is an empty list. Errors name the rule that failed.
func ParseUpstreamErrorRules(raw string) ([]UpstreamErrorRule, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var rules []UpstreamErrorRule
	if err := common.UnmarshalJsonStr(raw, &rules); err != nil {
		return nil, fmt.Errorf("upstream error rules must be a JSON array of rules: %w", err)
	}
	if len(rules) > maxUpstreamErrorRules {
		return nil, fmt.Errorf("at most %d upstream error rules are allowed", maxUpstreamErrorRules)
	}
	for i, rule := range rules {
		if err := validateUpstreamErrorRule(rule); err != nil {
			return nil, fmt.Errorf("upstream error rule %d: %w", i+1, err)
		}
	}
	return rules, nil
}

func validateUpstreamErrorRule(rule UpstreamErrorRule) error {
	if utf8.RuneCountInString(rule.Name) > maxUpstreamErrorNameRunes {
		return fmt.Errorf("name is longer than %d characters", maxUpstreamErrorNameRunes)
	}
	if _, err := ParseHTTPStatusCodeRanges(rule.StatusCodes); err != nil {
		return fmt.Errorf("status codes: %w", err)
	}
	if strings.TrimSpace(rule.StatusCodes) == "" && len(rule.Keywords) == 0 {
		return fmt.Errorf("set status codes, keywords or both")
	}
	if len(rule.Keywords) > maxUpstreamErrorKeywords {
		return fmt.Errorf("at most %d keywords are allowed", maxUpstreamErrorKeywords)
	}
	for i, keyword := range rule.Keywords {
		if strings.TrimSpace(keyword) == "" {
			return fmt.Errorf("keyword %d is empty", i+1)
		}
		if utf8.RuneCountInString(keyword) > maxUpstreamErrorKeywordRunes {
			return fmt.Errorf("keyword %d is longer than %d characters", i+1, maxUpstreamErrorKeywordRunes)
		}
	}
	switch rule.Action {
	case UpstreamErrorActionIntercept:
		return nil
	case UpstreamErrorActionRewrite:
	default:
		return fmt.Errorf("action must be %q or %q", UpstreamErrorActionIntercept, UpstreamErrorActionRewrite)
	}
	response := rule.Response
	if response == nil {
		return fmt.Errorf("a rewrite rule needs a response")
	}
	if response.Status < 400 || response.Status > 599 {
		return fmt.Errorf("response status must be between 400 and 599")
	}
	if !upstreamErrorCodePattern.MatchString(response.Code) {
		return fmt.Errorf("response code must be 1-64 letters, digits, dots, dashes or underscores")
	}
	if strings.TrimSpace(response.Message) == "" {
		return fmt.Errorf("response message is required")
	}
	if utf8.RuneCountInString(response.Message) > maxUpstreamErrorMessageRunes {
		return fmt.Errorf("response message is longer than %d characters", maxUpstreamErrorMessageRunes)
	}
	return nil
}

// EffectiveRules returns the rules in force: the saved rules, or the legacy
// fields as rules while none are saved. Saved rules that fail to parse match
// nothing; saving validates them, so they can only come from other writes.
func (s *UpstreamErrorSetting) EffectiveRules() []UpstreamErrorRule {
	if strings.TrimSpace(s.Rules) != "" {
		rules, err := ParseUpstreamErrorRules(s.Rules)
		if err != nil {
			common.SysError("invalid upstream error rules ignored: " + err.Error())
			return nil
		}
		return rules
	}
	var rules []UpstreamErrorRule
	if strings.TrimSpace(s.MaskStatusCodes) != "" {
		rules = append(rules, UpstreamErrorRule{
			Enabled:     s.MaskEnabled,
			StatusCodes: s.MaskStatusCodes,
			Keywords:    []string{},
			Action:      UpstreamErrorActionIntercept,
		})
	}
	keywords := []string{}
	for keyword := range strings.SplitSeq(s.MaskKeywords, "\n") {
		if keyword = strings.TrimSpace(keyword); keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	if len(keywords) > 0 {
		rules = append(rules, UpstreamErrorRule{
			Enabled:  s.MaskEnabled,
			Keywords: keywords,
			Action:   UpstreamErrorActionIntercept,
		})
	}
	return rules
}

// EffectiveRulesJSON is the rules option as the settings page shows it: the
// saved value unchanged, or the legacy fields as rules while none are saved.
func (s *UpstreamErrorSetting) EffectiveRulesJSON() string {
	if strings.TrimSpace(s.Rules) != "" {
		return s.Rules
	}
	encoded, err := common.Marshal(s.EffectiveRules())
	if err != nil {
		return "[]"
	}
	if string(encoded) == "null" {
		return "[]"
	}
	return string(encoded)
}

type compiledUpstreamErrorRule struct {
	position int
	rule     UpstreamErrorRule
	codes    []StatusCodeRange
	anyCode  bool
	keywords []string
}

type upstreamErrorRulesCache struct {
	setting UpstreamErrorSetting
	rules   []compiledUpstreamErrorRule
}

var cachedUpstreamErrorRules atomic.Pointer[upstreamErrorRulesCache]

// compiledRules reuses the compiled rules while the setting stays the same.
func (s *UpstreamErrorSetting) compiledRules() []compiledUpstreamErrorRule {
	current := *s
	if entry := cachedUpstreamErrorRules.Load(); entry != nil && entry.setting == current {
		return entry.rules
	}
	var compiled []compiledUpstreamErrorRule
	for i, rule := range current.EffectiveRules() {
		codes, err := ParseHTTPStatusCodeRanges(rule.StatusCodes)
		if err != nil {
			// A legacy status code list that never validated keeps
			// matching nothing, as it did before rules.
			continue
		}
		keywords := make([]string, 0, len(rule.Keywords))
		for _, keyword := range rule.Keywords {
			if keyword = strings.ToLower(strings.TrimSpace(keyword)); keyword != "" {
				keywords = append(keywords, keyword)
			}
		}
		compiled = append(compiled, compiledUpstreamErrorRule{
			position: i + 1,
			rule:     rule,
			codes:    codes,
			anyCode:  strings.TrimSpace(rule.StatusCodes) == "",
			keywords: keywords,
		})
	}
	cachedUpstreamErrorRules.Store(&upstreamErrorRulesCache{setting: current, rules: compiled})
	return compiled
}

// Match returns the first enabled rule that matches an upstream error with
// this status code and this error code and message text.
func (s *UpstreamErrorSetting) Match(statusCode int, text string) (UpstreamErrorRuleMatch, bool) {
	text = strings.ToLower(text)
	for _, compiled := range s.compiledRules() {
		if !compiled.rule.Enabled {
			continue
		}
		if !compiled.anyCode && !shouldMatchStatusCodeRanges(compiled.codes, statusCode) {
			continue
		}
		if len(compiled.keywords) > 0 && !slices.ContainsFunc(compiled.keywords, func(keyword string) bool {
			return strings.Contains(text, keyword)
		}) {
			continue
		}
		return UpstreamErrorRuleMatch{Position: compiled.position, Rule: compiled.rule}, true
	}
	return UpstreamErrorRuleMatch{}, false
}
