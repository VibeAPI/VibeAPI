package requestrule

import "github.com/QuantumNous/new-api/common"

const (
	openAIContentBlockTypes = `^(image_url|input_image|file|input_file)$`
	openAIOnlyParams        = `^(stop|max_completion_tokens|response_format|functions|function_call|stream_options|n|frequency_penalty|presence_penalty|logit_bias|logprobs|top_logprobs|seed|parallel_tool_calls)$`
	openAIOnlyMessageFields = `^(name|tool_calls|tool_call_id|function_call)$`
)

// Presets are the rules shipped with the gateway. Each answers an
// OpenAI-format request sent to the Claude Messages API, which Claude
// upstreams reject with 400. Names and descriptions are the frontend's i18n
// keys, so the settings page shows them translated until an admin edits them.
func Presets() []Rule {
	claude := []string{EndpointClaudeMessages}
	return []Rule{
		{
			ID:          "openai_content_blocks",
			Name:        "OpenAI image and file blocks",
			Description: "Content blocks of type image_url, input_image, file or input_file, including those inside tool results. Claude expects image and document blocks.",
			Preset:      true,
			Enabled:     true,
			Endpoints:   claude,
			Match:       MatchAny,
			Conditions: []Condition{
				{Source: SourceJSON, Path: "messages.#.content.#.type", Op: OpRegex, Value: openAIContentBlockTypes},
				{Source: SourceJSON, Path: "messages.#.content.#.content.#.type", Op: OpRegex, Value: openAIContentBlockTypes},
			},
			Response: Response{Status: 400, Code: DefaultErrorCode, Message: `request rule openai_content_blocks: {{field}} is an OpenAI "{{value}}" content block, which the Claude Messages API does not accept; use a Claude "image" or "document" block with a "source", or send OpenAI-format requests to /v1/chat/completions`},
		},
		{
			ID:          "openai_tools",
			Name:        "OpenAI-style tools",
			Description: "Tools written as type function with a nested function object. Claude expects name, description and input_schema on each tool.",
			Preset:      true,
			Enabled:     true,
			Endpoints:   claude,
			Match:       MatchAll,
			Conditions: []Condition{
				{Source: SourceJSON, Path: "tools.#.function", Op: OpExists},
			},
			Response: Response{Status: 400, Code: DefaultErrorCode, Message: `request rule openai_tools: {{field}} belongs to an OpenAI-style {"type":"function","function":{...}} tool, which the Claude Messages API does not accept; define Claude tools as {"name","description","input_schema"}, or send OpenAI-format requests to /v1/chat/completions`},
		},
		{
			ID:          "openai_tool_role",
			Name:        "Messages with role tool",
			Description: "Claude returns tool results as tool_result blocks inside a user message.",
			Preset:      true,
			Enabled:     true,
			Endpoints:   claude,
			Match:       MatchAll,
			Conditions: []Condition{
				{Source: SourceJSON, Path: "messages.#.role", Op: OpRegex, Value: `^tool$`},
			},
			Response: Response{Status: 400, Code: DefaultErrorCode, Message: `request rule openai_tool_role: {{field}} is the OpenAI role "tool", which the Claude Messages API does not accept; return tool results as "tool_result" blocks in a "user" message, or send OpenAI-format requests to /v1/chat/completions`},
		},
		{
			ID:              "openai_fields",
			Name:            "OpenAI-only parameters",
			Description:     "Parameters such as stop, stream_options, n, seed and response_format, and message fields such as name and tool_calls. Checked only when the channel passes the body through, because a rebuilt body drops them.",
			Preset:          true,
			Enabled:         true,
			Endpoints:       claude,
			OnlyPassThrough: true,
			Match:           MatchAny,
			Conditions: []Condition{
				{Source: SourceJSON, Path: "@keys", Op: OpRegex, Value: openAIOnlyParams},
				{Source: SourceJSON, Path: "messages.#.@keys", Op: OpRegex, Value: openAIOnlyMessageFields},
			},
			Response: Response{Status: 400, Code: DefaultErrorCode, Message: `request rule openai_fields: "{{value}}" is an OpenAI-only field, which the Claude Messages API does not accept; remove it or use the Claude equivalent (for example stop_sequences instead of stop), or send OpenAI-format requests to /v1/chat/completions`},
		},
		{
			ID:              "missing_max_tokens",
			Name:            "Missing max_tokens",
			Description:     "Claude requires max_tokens. Checked only when the channel passes the body through, because a rebuilt body gets a default value.",
			Preset:          true,
			Enabled:         true,
			Endpoints:       claude,
			OnlyPassThrough: true,
			Match:           MatchAll,
			Conditions: []Condition{
				{Source: SourceJSON, Path: "max_tokens", Op: OpNotExists},
			},
			Response: Response{Status: 400, Code: DefaultErrorCode, Message: `request rule missing_max_tokens: the Claude Messages API requires "max_tokens", and this channel relays the request body unchanged`},
		},
	}
}

// DefaultRulesJSON is the stored form of Presets, the default rule list.
func DefaultRulesJSON() string {
	data, err := common.Marshal(Presets())
	if err != nil {
		panic(err)
	}
	return string(data)
}
