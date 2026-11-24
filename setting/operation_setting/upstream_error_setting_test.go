package operation_setting

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUpstreamErrorRulesValidatesEachRule(t *testing.T) {
	rules, err := ParseUpstreamErrorRules(`[{"enabled":true,"status_codes":"500","keywords":["no upstream account token"],"action":"rewrite","response":{"status":429,"code":"rate_limit_exceeded","message":"Too many requests"},"skip_retry":true}]`)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	require.NotNil(t, rules[0].Response)
	assert.Equal(t, http.StatusTooManyRequests, rules[0].Response.Status)
	assert.True(t, rules[0].SkipRetry)

	rules, err = ParseUpstreamErrorRules("  ")
	require.NoError(t, err)
	assert.Empty(t, rules, "an empty value is an empty rule list")

	for name, raw := range map[string]string{
		"not an array":                `{"enabled":true,"status_codes":"500","action":"intercept"}`,
		"rule without a condition":    `[{"enabled":true,"action":"intercept"}]`,
		"reversed status code range":  `[{"enabled":true,"status_codes":"503-500","action":"intercept"}]`,
		"blank keyword":               `[{"enabled":true,"keywords":["quota"," "],"action":"intercept"}]`,
		"unknown action":              `[{"enabled":true,"status_codes":"500","action":"drop"}]`,
		"rewrite without a response":  `[{"enabled":true,"status_codes":"500","action":"rewrite"}]`,
		"rewrite to a success status": `[{"enabled":true,"status_codes":"500","action":"rewrite","response":{"status":200,"code":"ok","message":"fine"}}]`,
		"rewrite code with spaces":    `[{"enabled":true,"status_codes":"500","action":"rewrite","response":{"status":429,"code":"rate limit","message":"Too many requests"}}]`,
		"rewrite without a message":   `[{"enabled":true,"status_codes":"500","action":"rewrite","response":{"status":429,"code":"rate_limit_exceeded","message":" "}}]`,
	} {
		_, err := ParseUpstreamErrorRules(raw)
		assert.Error(t, err, name)
	}
}

func TestUpstreamErrorLegacySettingsReadAsInterceptRules(t *testing.T) {
	setting := UpstreamErrorSetting{MaskEnabled: false, MaskStatusCodes: "401-403", MaskKeywords: "余额不足\n\n insufficient balance \n"}
	assert.JSONEq(t, `[
		{"name":"","enabled":false,"status_codes":"401-403","keywords":[],"action":"intercept"},
		{"name":"","enabled":false,"status_codes":"","keywords":["余额不足","insufficient balance"],"action":"intercept"}
	]`, setting.EffectiveRulesJSON(), "unsaved rules show the legacy settings, turned off like the legacy switch")

	setting.MaskEnabled = true
	match, ok := setting.Match(http.StatusInternalServerError, "上游余额不足")
	require.True(t, ok)
	assert.Equal(t, 2, match.Position)

	setting.Rules = `[]`
	assert.Equal(t, `[]`, setting.EffectiveRulesJSON())
	_, ok = setting.Match(http.StatusUnauthorized, "invalid token")
	assert.False(t, ok, "saved rules replace the legacy settings")
}
