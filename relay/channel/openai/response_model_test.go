package openai_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponseHandlersHonorRequestedModelName(t *testing.T) {
	oldMode, oldTimeout := gin.Mode(), constant.StreamingTimeout
	gin.SetMode(gin.TestMode)
	constant.StreamingTimeout = 30
	t.Cleanup(func() { gin.SetMode(oldMode); constant.StreamingTimeout = oldTimeout })
	const responseJSON = `{"id":"resp_1","model":"provider-model","status":"completed","output":[],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3},"vendor_extension":{"trace_id":"trace-1"}}`
	const chatJSON = `{"id":"chat_1","model":"provider-model","choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`
	const claudeJSON = `{"id":"msg_1","type":"message","role":"assistant","model":"provider-model","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":1}}`
	const geminiJSON = `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":1,"totalTokenCount":3}}`
	responseStream := "data: " + `{"type":"response.created","response":{"id":"resp_1","model":"provider-model","output":[]}}` + "\n\ndata: " + `{"type":"response.completed","response":` + responseJSON + "}\n\ndata: [DONE]\n\n"
	chatStream := "data: " + `{"id":"chat_1","model":"provider-model","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"},"finish_reason":null}]}` + "\n\ndata: " + `{"id":"chat_1","model":"provider-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}` + "\n\ndata: [DONE]\n\n"
	claudeStream := "event: message_start\ndata: " + `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"provider-model","content":[],"usage":{"input_tokens":2,"output_tokens":0}}}` + "\n\nevent: content_block_delta\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}` + "\n\nevent: message_delta\ndata: " + `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}` + "\n\nevent: message_stop\ndata: " + `{"type":"message_stop"}` + "\n\n"
	for _, tc := range []struct {
		name, body string
		format     types.RelayFormat
		stream     bool
		handler    func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)
	}{
		{"responses JSON", responseJSON, types.RelayFormatOpenAIResponses, false, openai.OaiResponsesHandler},
		{"responses stream", responseStream, types.RelayFormatOpenAIResponses, true, openai.OaiResponsesStreamHandler},
		{"chat to responses JSON", chatJSON, types.RelayFormatOpenAIResponses, false, openai.OaiChatToResponsesHandler},
		{"chat to responses stream", chatStream, types.RelayFormatOpenAIResponses, true, openai.OaiChatToResponsesStreamHandler},
		{"gemini to responses JSON", geminiJSON, types.RelayFormatOpenAIResponses, false, gemini.GeminiResponsesHandler},
		{"gemini to responses stream", "data: " + geminiJSON + "\n\n", types.RelayFormatOpenAIResponses, true, gemini.GeminiResponsesStreamHandler},
		{"claude JSON", claudeJSON, types.RelayFormatClaude, false, func(c *gin.Context, i *relaycommon.RelayInfo, r *http.Response) (*dto.Usage, *types.NewAPIError) {
			return claude.ClaudeHandler(c, r, i)
		}},
		{"claude stream", claudeStream, types.RelayFormatClaude, true, func(c *gin.Context, i *relaycommon.RelayInfo, r *http.Response) (*dto.Usage, *types.NewAPIError) {
			return claude.ClaudeStreamHandler(c, r, i)
		}},
		{"claude to responses JSON", claudeJSON, types.RelayFormatOpenAIResponses, false, func(c *gin.Context, i *relaycommon.RelayInfo, r *http.Response) (*dto.Usage, *types.NewAPIError) {
			return claude.ClaudeHandler(c, r, i)
		}},
		{"claude to responses stream", claudeStream, types.RelayFormatOpenAIResponses, true, func(c *gin.Context, i *relaycommon.RelayInfo, r *http.Response) (*dto.Usage, *types.NewAPIError) {
			return claude.ClaudeResponsesStreamHandler(c, r, i)
		}},
	} {
		for _, enabled := range []bool{false, true} {
			name := "disabled"
			if enabled {
				name = "enabled"
			}
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				info := &relaycommon.RelayInfo{OriginModelName: "client-alias", RelayFormat: tc.format, IsStream: tc.stream, DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "provider-model", ChannelSetting: dto.ChannelSettings{ReturnRequestModelName: enabled}}}
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(tc.body))}
				if tc.stream {
					resp.Header.Set("Content-Type", "text/event-stream")
				}
				usage, apiErr := tc.handler(c, info, resp)
				require.Nil(t, apiErr)
				require.NotNil(t, usage)
				want := "provider-model"
				if enabled {
					want = "client-alias"
				}
				payloads := []string{w.Body.String()}
				if tc.stream {
					payloads = nil
					for line := range strings.SplitSeq(w.Body.String(), "\n") {
						if data, ok := strings.CutPrefix(line, "data: "); ok {
							payloads = append(payloads, data)
						}
					}
				}
				models := 0
				for _, payload := range payloads {
					for _, path := range []string{"model", "message.model", "response.model"} {
						if value := gjson.Get(payload, path); value.Exists() {
							models++
							assert.Equal(t, want, value.String())
						}
					}
				}
				assert.Positive(t, models, "response must contain a model")
				if tc.name == "responses JSON" {
					assert.Equal(t, "trace-1", gjson.Get(w.Body.String(), "vendor_extension.trace_id").String())
				}
			})
		}
	}
}
