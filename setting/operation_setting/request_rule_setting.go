package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// RequestRuleSetting selects the request rules that channels with request
// rules turned on apply to Claude Messages (/v1/messages) requests before
// relaying. Each rule matches an OpenAI-format request that Claude upstreams
// reject, so a match is answered locally with 400 instead of being relayed.
type RequestRuleSetting struct {
	// OpenAIContentBlocks matches image_url, input_image, file and input_file
	// content blocks, including those inside tool_result content.
	OpenAIContentBlocks bool `json:"openai_content_blocks"`
	// OpenAITools matches tools written as {"type":"function","function":{...}}.
	OpenAITools bool `json:"openai_tools"`
	// OpenAIToolRole matches messages with role "tool".
	OpenAIToolRole bool `json:"openai_tool_role"`
	// OpenAIFields matches OpenAI-only top-level parameters such as stop and
	// stream_options, and OpenAI message fields such as name. It applies only
	// when the body is passed through, because a rebuilt body drops them.
	OpenAIFields bool `json:"openai_fields"`
	// MissingMaxTokens matches requests without max_tokens. It applies only
	// when the body is passed through, because a rebuilt body gets a default.
	MissingMaxTokens bool `json:"missing_max_tokens"`
}

var requestRuleSetting = RequestRuleSetting{
	OpenAIContentBlocks: true,
	OpenAITools:         true,
	OpenAIToolRole:      true,
	OpenAIFields:        true,
	MissingMaxTokens:    true,
}

func init() {
	config.GlobalConfig.Register("request_rule_setting", &requestRuleSetting)
}

func GetRequestRuleSetting() *RequestRuleSetting {
	return &requestRuleSetting
}
