package requestrule

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The presets answer the OpenAI-format requests that Claude upstreams reject.
// Fields that a rebuilt body drops or fills in are checked only when the
// channel passes the body through, and Claude request shapes always pass.
func TestPresetsMatchOpenAIFormatClaudeRequests(t *testing.T) {
	rules, err := Parse(DefaultRulesJSON())
	require.NoError(t, err)
	require.Equal(t, 5, rules.Len())

	for _, tc := range []struct {
		name        string
		body        string
		passThrough bool
		wantID      string
		wantMessage string
	}{
		{
			name:        "image_url block",
			body:        `{"model":"claude-haiku-5-5","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"what is this"},{"type":"image_url","image_url":{"url":"https://example.com/cat.png"}}]}]}`,
			wantID:      "openai_content_blocks",
			wantMessage: `request rule openai_content_blocks: messages[0].content[1].type is an OpenAI "image_url" content block`,
		},
		{
			name: "input_file inside a tool result",
			body: `{"model":"claude-sonnet-5","max_tokens":64,"messages":[
				{"role":"user","content":"take a screenshot"},
				{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"screenshot","input":{}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"done"},{"type":"input_file","file_url":"https://example.com/a.pdf"}]}]}]}`,
			wantID:      "openai_content_blocks",
			wantMessage: `messages[2].content[0].content[1].type is an OpenAI "input_file" content block`,
		},
		{
			name:        "OpenAI function tool",
			body:        `{"model":"claude-sonnet-5","max_tokens":64,"messages":[{"role":"user","content":"weather?"}],"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object"}}}]}`,
			wantID:      "openai_tools",
			wantMessage: `request rule openai_tools: tools[0].function belongs to an OpenAI-style`,
		},
		{
			name:        "OpenAI tool role",
			body:        `{"model":"claude-sonnet-5","max_tokens":64,"messages":[{"role":"user","content":"weather?"},{"role":"assistant","content":"checking"},{"role":"tool","tool_call_id":"call_1","content":"sunny"}]}`,
			wantID:      "openai_tool_role",
			wantMessage: `messages[2].role is the OpenAI role "tool"`,
		},
		{
			name:        "stop parameter when passed through",
			body:        `{"model":"claude-opus-5-5","max_tokens":64,"stream":true,"stop":["END"],"messages":[{"role":"user","content":"hi"}]}`,
			passThrough: true,
			wantID:      "openai_fields",
			wantMessage: `request rule openai_fields: "stop" is an OpenAI-only field`,
		},
		{
			name:        "message name when passed through",
			body:        `{"model":"claude-opus-5-5","max_tokens":64,"messages":[{"role":"user","content":"hi"},{"role":"assistant","name":"planner","content":"ok"}]}`,
			passThrough: true,
			wantID:      "openai_fields",
			wantMessage: `"name" is an OpenAI-only field`,
		},
		{
			name:        "missing max_tokens when passed through",
			body:        `{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hi"}]}`,
			passThrough: true,
			wantID:      "missing_max_tokens",
			wantMessage: `the Claude Messages API requires "max_tokens"`,
		},
		{
			name: "OpenAI fields and missing max_tokens pass when the body is rebuilt",
			body: `{"model":"claude-opus-5-5","stop":["END"],"stream_options":{"include_usage":true},"messages":[{"role":"user","name":"u1","content":"hi"}]}`,
		},
		{
			name: "Claude request passes when passed through",
			body: `{"model":"claude-sonnet-5","max_tokens":64,"stop_sequences":["END"],"metadata":{"user_id":"u1"},"messages":[
				{"role":"user","content":[{"type":"text","text":"compare"},{"type":"image","source":{"type":"url","url":"https://example.com/cat.png"}},{"type":"document","source":{"type":"url","url":"https://example.com/a.pdf"}},{"type":"future_block_20991231","payload":{}}]},
				{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"screenshot","input":{}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"image","source":{"type":"url","url":"https://example.com/shot.png"}}]}]}],
				"tools":[{"name":"screenshot","input_schema":{"type":"object"}},{"type":"web_search_20250305","name":"web_search"}]}`,
			passThrough: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match := rules.Match(Input{Endpoint: EndpointClaudeMessages, Model: "claude-opus-5-5", PassThrough: tc.passThrough, Body: []byte(tc.body)})
			if tc.wantID == "" {
				assert.Nil(t, match)
				return
			}
			require.NotNil(t, match)
			assert.Equal(t, tc.wantID, match.Rule.ID)
			assert.Contains(t, match.Message, tc.wantMessage)
			assert.Nil(t, rules.Match(Input{Endpoint: EndpointOpenAIChat, Model: "claude-opus-5-5", PassThrough: tc.passThrough, Body: []byte(tc.body)}), "presets apply to /v1/messages only")
		})
	}
}

// Custom rules combine the model regex, endpoints and conditions the way the
// settings page describes them, and the error message fills in variables.
func TestCustomRules(t *testing.T) {
	rules, err := Parse(`[
		{"id":"prefill","name":"Prefill","enabled":true,"endpoints":["claude_messages"],"model_regex":"^claude-sonnet-4-6","match":"all",
		 "conditions":[{"source":"json","path":"messages.-1.role","op":"regex","value":"^assistant$"}],
		 "response":{"status":400,"code":"no_prefill","message":"{{model}} does not support prefill ({{field}} = {{value}}, rule {{rule}}, {{path}})"}},
		{"id":"clear_thinking","name":"clear_thinking","enabled":true,"endpoints":["claude_messages"],"match":"all",
		 "conditions":[{"source":"json","path":"context_management.edits.#.type","op":"regex","value":"^clear_thinking_"},
		               {"source":"json","path":"thinking.type","op":"not_regex","value":"^(enabled|adaptive)$"}],
		 "response":{"status":400,"code":"request_rule_blocked","message":"turn on thinking"}},
		{"id":"effort_none","name":"effort none","enabled":true,"endpoints":["openai_chat","openai_responses"],"model_regex":"^gpt-6","match":"any",
		 "conditions":[{"source":"json","path":"reasoning_effort","op":"regex","value":"^none$"},
		               {"source":"json","path":"reasoning.effort","op":"regex","value":"^none$"}],
		 "response":{"status":422,"code":"effort_none","message":"{{model}}: {{field}} cannot be none"}},
		{"id":"beta_header","name":"beta header","enabled":true,"endpoints":["gemini"],"match":"all",
		 "conditions":[{"source":"body","op":"regex","value":"secret-[0-9]+"},{"source":"header","path":"X-Debug","op":"exists"}],
		 "response":{"status":403,"code":"debug_blocked","message":"blocked {{value}}"}}
	]`)
	require.NoError(t, err)

	for _, tc := range []struct {
		name        string
		in          Input
		wantID      string
		wantField   string
		wantMessage string
	}{
		{
			name:        "last message from the assistant",
			in:          Input{Endpoint: EndpointClaudeMessages, Model: "claude-sonnet-4-6", Path: "/v1/messages", Body: []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"Translation:"}]}`)},
			wantID:      "prefill",
			wantField:   "messages[1].role",
			wantMessage: "claude-sonnet-4-6 does not support prefill (messages[1].role = assistant, rule Prefill, /v1/messages)",
		},
		{
			name: "prefill rule skips other models",
			in:   Input{Endpoint: EndpointClaudeMessages, Model: "claude-opus-5-5", Body: []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"Translation:"}]}`)},
		},
		{
			name:      "clear_thinking without thinking",
			in:        Input{Endpoint: EndpointClaudeMessages, Model: "claude-opus-4-8", Body: []byte(`{"messages":[{"role":"user","content":"hi"}],"context_management":{"edits":[{"type":"clear_thinking_20251015"}]}}`)},
			wantID:    "clear_thinking",
			wantField: "context_management.edits[0].type",
		},
		{
			name: "clear_thinking with adaptive thinking passes",
			in:   Input{Endpoint: EndpointClaudeMessages, Model: "claude-opus-4-8", Body: []byte(`{"messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"context_management":{"edits":[{"type":"clear_thinking_20251015"}]}}`)},
		},
		{
			name:        "effort none on chat",
			in:          Input{Endpoint: EndpointOpenAIChat, Model: "gpt-6-astra", Body: []byte(`{"model":"gpt-6-astra","reasoning_effort":"none","messages":[]}`)},
			wantID:      "effort_none",
			wantField:   "reasoning_effort",
			wantMessage: "gpt-6-astra: reasoning_effort cannot be none",
		},
		{
			name:      "effort none on responses",
			in:        Input{Endpoint: EndpointOpenAIResponses, Model: "gpt-6.1-sol", Body: []byte(`{"model":"gpt-6.1-sol","reasoning":{"effort":"none"},"input":"hi"}`)},
			wantID:    "effort_none",
			wantField: "reasoning.effort",
		},
		{
			name: "effort none rule skips Claude requests",
			in:   Input{Endpoint: EndpointClaudeMessages, Model: "gpt-6-astra", Body: []byte(`{"reasoning_effort":"none"}`)},
		},
		{
			name:        "header and body conditions",
			in:          Input{Endpoint: EndpointGemini, Model: "gemini-4", Header: http.Header{"X-Debug": {"1"}}, Body: []byte(`{"contents":[{"parts":[{"text":"use secret-42"}]}]}`)},
			wantID:      "beta_header",
			wantField:   "body",
			wantMessage: "blocked secret-42",
		},
		{
			name: "header condition fails without the header",
			in:   Input{Endpoint: EndpointGemini, Model: "gemini-4", Body: []byte(`{"contents":[{"parts":[{"text":"use secret-42"}]}]}`)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match := rules.Match(tc.in)
			if tc.wantID == "" {
				assert.Nil(t, match)
				return
			}
			require.NotNil(t, match)
			assert.Equal(t, tc.wantID, match.Rule.ID)
			assert.Equal(t, tc.wantField, match.Field)
			if tc.wantMessage != "" {
				assert.Equal(t, tc.wantMessage, match.Message)
			}
		})
	}
}

func TestExplainReportsWhyEachRuleDidNotMatch(t *testing.T) {
	rules, err := Parse(`[
		{"id":"off","name":"off","enabled":false,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}},
		{"id":"chat","name":"chat","enabled":true,"endpoints":["openai_chat"],"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}},
		{"id":"other_model","name":"other model","enabled":true,"endpoints":["claude_messages"],"model_regex":"^gpt","conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}},
		{"id":"pass","name":"pass","enabled":true,"endpoints":["claude_messages"],"only_pass_through":true,"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}},
		{"id":"temp","name":"temp","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"temperature","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}},
		{"id":"hit","name":"hit","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"model","op":"regex","value":"^claude"}],"response":{"status":400,"code":"x","message":"m"}},
		{"id":"after","name":"after","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}}
	]`)
	require.NoError(t, err)
	match, traces := rules.Explain(Input{Endpoint: EndpointClaudeMessages, Model: "claude-opus-5-5", Body: []byte(`{"model":"claude-opus-5-5"}`)})
	require.NotNil(t, match)
	assert.Equal(t, "hit", match.Rule.ID)
	results := make([]string, 0, len(traces))
	for _, trace := range traces {
		results = append(results, trace.Result)
	}
	assert.Equal(t, []string{TraceDisabled, TraceEndpoint, TraceModel, TracePassThrough, TraceConditions, TraceMatched, TraceNotChecked}, results)
	assert.Equal(t, 6, traces[5].Position)
}

func TestParseRejectsInvalidRules(t *testing.T) {
	const valid = `"enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}`
	for _, tc := range []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"not an array", `{"id":"a"}`, "must be a JSON array"},
		{"bad regex names the rule and condition", `[{"id":"a","name":"Prefill","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"model","op":"exists"},{"source":"json","path":"messages.-1.role","op":"regex","value":"(?=assistant)"}],"response":{"status":400,"code":"x","message":"m"}}]`, "rule 1 (Prefill): condition 2: invalid regex"},
		{"no endpoint", `[{"id":"a","name":"a","enabled":true,"endpoints":[],"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}}]`, "select at least one endpoint"},
		{"unknown endpoint", `[{"id":"a","name":"a","enabled":true,"endpoints":["embeddings"],"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}}]`, `unknown endpoint "embeddings"`},
		{"status outside 4xx", `[{"id":"a","name":"a","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":500,"code":"x","message":"m"}}]`, "between 400 and 499"},
		{"json condition without a path", `[{"id":"a","name":"a","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}}]`, "path must be"},
		{"@keys in the middle", `[{"id":"a","name":"a","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"@keys.x","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}}]`, "@keys must be the last segment"},
		{"exists on the body", `[{"id":"a","name":"a","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"body","op":"exists"}],"response":{"status":400,"code":"x","message":"m"}}]`, "only be matched with a regex"},
		{"empty message", `[{"id":"a","name":"a","enabled":true,"endpoints":["claude_messages"],"conditions":[{"source":"json","path":"model","op":"exists"}],"response":{"status":400,"code":"x","message":" "}}]`, "error message must be"},
		{"duplicate id", `[{"id":"a","name":"a",` + valid + `},{"id":"a","name":"b",` + valid + `}]`, `rule 2 (b): duplicate id "a"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
	rules, err := Parse("")
	require.NoError(t, err)
	assert.Zero(t, rules.Len(), "an empty setting is an empty rule list")
}
