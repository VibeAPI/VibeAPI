package helper

import (
	"errors"
	"fmt"
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// CheckClaudeRequestRules returns an error naming the first enabled request
// rule that a Claude Messages request breaks. Every rule matches an
// OpenAI-format request sent to /v1/messages, which Claude upstreams reject.
// The openai_fields and missing_max_tokens rules apply only when passThrough
// is set, because a rebuilt body drops OpenAI-only fields and gets a default
// max_tokens.
func CheckClaudeRequestRules(c *gin.Context, request *dto.ClaudeRequest, passThrough bool) error {
	rules := operation_setting.GetRequestRuleSetting()
	if rules.OpenAIContentBlocks {
		for i, message := range request.Messages {
			if path, partType := openAIContentPartIn(message.Content); partType != "" {
				return fmt.Errorf(`request rule openai_content_blocks: messages[%d].content%s is an OpenAI %q block, which the Claude Messages API does not accept; use a Claude %q block with a "source", or send OpenAI-format requests to /v1/chat/completions`, i, path, partType, openAIContentPartClaudeBlock[partType])
			}
		}
	}
	if tools, ok := request.Tools.([]any); ok && rules.OpenAITools {
		for i, item := range tools {
			tool, _ := item.(map[string]any)
			if _, hasFunction := tool["function"]; hasFunction && tool["type"] == "function" {
				return fmt.Errorf(`request rule openai_tools: tools[%d] is an OpenAI-style {"type":"function","function":{...}} tool, which the Claude Messages API does not accept; define Claude tools as {"name","description","input_schema"}, or send OpenAI-format requests to /v1/chat/completions`, i)
			}
		}
	}
	if rules.OpenAIToolRole {
		for i, message := range request.Messages {
			if message.Role == "tool" {
				return fmt.Errorf(`request rule openai_tool_role: messages[%d] has the OpenAI role "tool", which the Claude Messages API does not accept; return tool results as "tool_result" blocks in a "user" message, or send OpenAI-format requests to /v1/chat/completions`, i)
			}
		}
	}
	if !passThrough {
		return nil
	}
	if rules.MissingMaxTokens && request.MaxTokens == nil {
		return errors.New(`request rule missing_max_tokens: the Claude Messages API requires "max_tokens", and this channel relays the request body unchanged`)
	}
	if !rules.OpenAIFields {
		return nil
	}
	// The parsed request does not keep unknown fields, so read the body that
	// will be relayed. GetBytes scans it in place and @keys returns only key
	// names, so a large body is not copied.
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil
	}
	for _, key := range gjson.GetBytes(body, "@keys").Array() {
		replacement, isOpenAI := openAIOnlyParams[key.Str]
		if !isOpenAI {
			continue
		}
		fix := "remove it"
		if replacement != "" {
			fix = fmt.Sprintf("use %q instead", replacement)
		}
		return fmt.Errorf(`request rule openai_fields: %q is an OpenAI Chat Completions parameter, which the Claude Messages API does not accept; %s, or send OpenAI-format requests to /v1/chat/completions`, key.Str, fix)
	}
	for i, keys := range gjson.GetBytes(body, "messages.#.@keys").Array() {
		for _, key := range keys.Array() {
			if slices.Contains(openAIOnlyMessageFields, key.Str) {
				return fmt.Errorf(`request rule openai_fields: messages[%d] has the OpenAI message field %q, which the Claude Messages API does not accept; remove it, or send OpenAI-format requests to /v1/chat/completions`, i, key.Str)
			}
		}
	}
	return nil
}

// openAIContentPartClaudeBlock maps OpenAI Chat Completions and Responses
// media part types to the Claude block that carries the same data. Claude has
// no block types with these names. Other unknown types are not matched,
// because Anthropic keeps adding new block types.
var openAIContentPartClaudeBlock = map[string]string{
	"image_url":   "image",
	"input_image": "image",
	"file":        "document",
	"input_file":  "document",
}

// openAIOnlyParams maps OpenAI Chat Completions parameters that the Claude
// Messages API rejects to the Claude parameter that replaces them, if any.
var openAIOnlyParams = map[string]string{
	"stop":                  "stop_sequences",
	"max_completion_tokens": "max_tokens",
	"response_format":       "output_format",
	"functions":             "tools",
	"function_call":         "tool_choice",
	"stream_options":        "",
	"n":                     "",
	"frequency_penalty":     "",
	"presence_penalty":      "",
	"logit_bias":            "",
	"logprobs":              "",
	"top_logprobs":          "",
	"seed":                  "",
	"parallel_tool_calls":   "",
}

// openAIOnlyMessageFields are OpenAI chat message fields that Claude messages
// do not have.
var openAIOnlyMessageFields = []string{"name", "tool_calls", "tool_call_id", "function_call"}

// openAIContentPartIn returns the index path and type of the first OpenAI
// media part in a Claude content array, including tool_result content.
func openAIContentPartIn(content any) (path string, partType string) {
	blocks, ok := content.([]any)
	if !ok {
		return "", ""
	}
	for i, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := block["type"].(string)
		if _, isOpenAI := openAIContentPartClaudeBlock[blockType]; isOpenAI {
			return fmt.Sprintf("[%d]", i), blockType
		}
		if blockType != "tool_result" {
			continue
		}
		if nestedPath, nestedType := openAIContentPartIn(block["content"]); nestedType != "" {
			return fmt.Sprintf("[%d].content%s", i, nestedPath), nestedType
		}
	}
	return "", ""
}
