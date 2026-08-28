package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPromptAuditContentUsesLatestUserInputAndRedactsCredentials(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{Messages: []dto.Message{
		{Role: "user", Content: "old request"},
		{Role: "assistant", Content: "tool result bearer secret-token-123"},
		{Role: "user", Content: "latest request api_key=sk-super-secret-value"},
	}}
	setting := operation_setting.PromptAuditSetting{ContentScope: operation_setting.PromptAuditContentLatestTools, MaxCharacters: 40000}

	content, truncated := ExtractPromptAuditContent(request, setting)

	require.NotEmpty(t, content)
	assert.Contains(t, content, "latest request")
	assert.NotContains(t, content, "old request")
	assert.NotContains(t, content, "sk-super-secret-value")
	assert.False(t, truncated)
}

func TestExtractPromptAuditContentIncludesOnlyToolResultsAfterLatestUser(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{Messages: []dto.Message{
		{Role: "user", Content: "old request"},
		{Role: "tool", Content: "old tool result"},
		{Role: "assistant", Content: "old assistant response"},
		{Role: "user", Content: "latest request"},
		{Role: "assistant", Content: "assistant planning"},
		{Role: "tool", Content: "new tool result"},
	}}
	setting := operation_setting.PromptAuditSetting{ContentScope: operation_setting.PromptAuditContentLatestTools, MaxCharacters: 40000}

	content, truncated := ExtractPromptAuditContent(request, setting)

	assert.Equal(t, "latest request\nnew tool result", content)
	assert.False(t, truncated)
}

func TestValidatePromptAuditEndpointAlwaysBlocksMetadataAddresses(t *testing.T) {
	for _, endpoint := range []string{
		"https://169.254.169.254/v1/chat/completions",
		"https://100.100.100.200/v1/chat/completions",
	} {
		assert.Error(t, ValidatePromptAuditEndpoint(endpoint, true))
	}
}

func TestPromptAuditAudienceModes(t *testing.T) {
	setting := operation_setting.PromptAuditSetting{AudienceUserIds: []int{7}}
	setting.AudienceMode = operation_setting.PromptAuditScopeAll
	assert.True(t, ShouldPromptAuditUser(setting, 8))
	setting.AudienceMode = operation_setting.PromptAuditScopeWhitelist
	assert.False(t, ShouldPromptAuditUser(setting, 7))
	assert.True(t, ShouldPromptAuditUser(setting, 8))
	setting.AudienceMode = operation_setting.PromptAuditScopeBlacklist
	assert.True(t, ShouldPromptAuditUser(setting, 7))
	assert.False(t, ShouldPromptAuditUser(setting, 8))
}
