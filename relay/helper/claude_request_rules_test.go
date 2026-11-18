package helper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Request rules answer OpenAI-format requests sent to /v1/messages locally.
// Rules for fields the gateway drops or fills when it rebuilds the body only
// apply when the body is passed through, and Claude request shapes, including
// block types the gateway does not know yet, always pass.
func TestCheckClaudeRequestRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rules := operation_setting.GetRequestRuleSetting()
	allRules := *rules
	t.Cleanup(func() { *rules = allRules })
	toolsRuleOff := allRules
	toolsRuleOff.OpenAITools = false

	const openAITools = `{"model":"claude-sonnet-5","max_tokens":64,"messages":[{"role":"user","content":"weather?"}],"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object"}}}]}`
	for _, tc := range []struct {
		name        string
		body        string
		passThrough bool
		setting     *operation_setting.RequestRuleSetting
		wantErr     string
	}{
		{
			name:    "openai image_url block",
			body:    `{"model":"claude-haiku-5-5","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"what is this"},{"type":"image_url","image_url":{"url":"https://example.com/cat.png"}}]}]}`,
			wantErr: `request rule openai_content_blocks: messages[0].content[1] is an OpenAI "image_url" block, which the Claude Messages API does not accept; use a Claude "image" block`,
		},
		{
			name: "openai image inside tool_result",
			body: `{"model":"claude-sonnet-5","max_tokens":64,"messages":[
				{"role":"user","content":"take a screenshot"},
				{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"screenshot","input":{}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"done"},{"type":"input_file","file_url":"https://example.com/a.pdf"}]}]}]}`,
			wantErr: `request rule openai_content_blocks: messages[2].content[0].content[1] is an OpenAI "input_file" block, which the Claude Messages API does not accept; use a Claude "document" block`,
		},
		{
			name:    "openai function tool",
			body:    openAITools,
			wantErr: `request rule openai_tools: tools[0] is an OpenAI-style {"type":"function","function":{...}} tool`,
		},
		{
			name:    "openai tool role",
			body:    `{"model":"claude-sonnet-5","max_tokens":64,"messages":[{"role":"user","content":"weather?"},{"role":"assistant","content":"checking"},{"role":"tool","tool_call_id":"call_1","content":"sunny"}]}`,
			wantErr: `request rule openai_tool_role: messages[2] has the OpenAI role "tool"`,
		},
		{
			name:        "openai stop parameter when passed through",
			body:        `{"model":"claude-opus-5-5","max_tokens":64,"stream":true,"stop":["END"],"messages":[{"role":"user","content":"hi"}]}`,
			passThrough: true,
			wantErr:     `request rule openai_fields: "stop" is an OpenAI Chat Completions parameter, which the Claude Messages API does not accept; use "stop_sequences" instead`,
		},
		{
			name:        "openai message name when passed through",
			body:        `{"model":"claude-opus-5-5","max_tokens":64,"messages":[{"role":"user","content":"hi"},{"role":"assistant","name":"planner","content":"ok"}]}`,
			passThrough: true,
			wantErr:     `request rule openai_fields: messages[1] has the OpenAI message field "name"`,
		},
		{
			name:        "missing max_tokens when passed through",
			body:        `{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hi"}]}`,
			passThrough: true,
			wantErr:     `request rule missing_max_tokens`,
		},
		{
			name: "openai fields and missing max_tokens pass when the body is rebuilt",
			body: `{"model":"claude-opus-5-5","stop":["END"],"stream_options":{"include_usage":true},"messages":[{"role":"user","name":"u1","content":"hi"}]}`,
		},
		{
			name:    "disabled rule is skipped",
			body:    openAITools,
			setting: &toolsRuleOff,
		},
		{
			name: "claude request passes when passed through",
			body: `{"model":"claude-sonnet-5","max_tokens":64,"stop_sequences":["END"],"metadata":{"user_id":"u1"},"messages":[
				{"role":"user","content":[{"type":"text","text":"compare"},{"type":"image","source":{"type":"url","url":"https://example.com/cat.png"}},{"type":"document","source":{"type":"url","url":"https://example.com/a.pdf"}},{"type":"future_block_20991231","payload":{}}]},
				{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"screenshot","input":{}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"image","source":{"type":"url","url":"https://example.com/shot.png"}}]}]}],
				"tools":[{"name":"screenshot","input_schema":{"type":"object"}},{"type":"web_search_20250305","name":"web_search"},{"type":"mcp_toolset","mcp_server_name":"docs"}]}`,
			passThrough: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			*rules = allRules
			if tc.setting != nil {
				*rules = *tc.setting
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			request, err := GetAndValidateClaudeRequest(c)
			require.NoError(t, err)

			err = CheckClaudeRequestRules(c, request, tc.passThrough)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
