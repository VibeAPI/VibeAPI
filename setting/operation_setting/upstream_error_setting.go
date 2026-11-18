package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// UpstreamErrorSetting decides which upstream errors callers see as the
// gateway's own "no available channel" error instead of the upstream text.
// They describe the upstream account rather than the caller's request:
// rejected credentials, exhausted quota or balance (also when relayed from
// further upstream), and models the upstream cannot serve.
type UpstreamErrorSetting struct {
	MaskEnabled bool `json:"mask_enabled"`
	// MaskStatusCodes always mask: the caller's own key and balance are
	// checked before relaying, so these statuses concern the upstream account.
	MaskStatusCodes string `json:"mask_status_codes"`
	// MaskKeywords, one per line, match the upstream error code or message
	// case-insensitively.
	MaskKeywords string `json:"mask_keywords"`
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

// ShouldMask reports whether an upstream error with this status code and this
// error code and message text is shown to callers as the gateway's own error.
func (s *UpstreamErrorSetting) ShouldMask(statusCode int, text string) bool {
	if !s.MaskEnabled {
		return false
	}
	if ranges, err := ParseHTTPStatusCodeRanges(s.MaskStatusCodes); err == nil && shouldMatchStatusCodeRanges(ranges, statusCode) {
		return true
	}
	text = strings.ToLower(text)
	for keyword := range strings.SplitSeq(s.MaskKeywords, "\n") {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}
