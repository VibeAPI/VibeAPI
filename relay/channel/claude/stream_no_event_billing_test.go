package claude

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noEventFailedReader struct{}

func (noEventFailedReader) Read([]byte) (int, error) {
	return 0, errors.New("upstream connection reset")
}

type noEventCancelReader struct {
	reader *io.PipeReader
	cancel context.CancelFunc
}

func (r *noEventCancelReader) Read(p []byte) (int, error) {
	r.cancel()
	return r.reader.Read(p)
}

func (r *noEventCancelReader) Close() error { return r.reader.Close() }

const testEstimatedPromptTokens = 545667

// A stream that never delivered a non-heartbeat upstream event has no usage
// basis, so it must fail (and be refunded) instead of billing the locally
// estimated prompt tokens. Streams that delivered real events keep billing.
func TestClaudeStreamHandlerNoUpstreamEventIsNotBilled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })

	messageStart := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-fable-5-1\",\"content\":[],\"usage\":{\"input_tokens\":1166,\"output_tokens\":1,\"cache_creation_input_tokens\":707431}}}\n\n"
	fullStream := messageStart +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":1166,\"output_tokens\":12,\"cache_creation_input_tokens\":707431}}\n\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	tests := []struct {
		name          string
		stream        string
		body          func(cancel context.CancelFunc) (io.ReadCloser, func())
		wantErr       bool
		wantEnd       relaycommon.StreamEndReason
		wantPrompt    int
		wantOutput    int
		wantCacheMake int
	}{
		{name: "empty HTTP 200 stream", wantErr: true, wantEnd: relaycommon.StreamEndReasonEOF},
		{name: "heartbeat only", stream: "event: ping\ndata: {\"type\":\"ping\"}\n\n", wantErr: true, wantEnd: relaycommon.StreamEndReasonEOF},
		{
			name: "connection reset before output", wantErr: true, wantEnd: relaycommon.StreamEndReasonScannerErr,
			body: func(context.CancelFunc) (io.ReadCloser, func()) { return io.NopCloser(noEventFailedReader{}), func() {} },
		},
		{
			name: "client disconnected before output", wantErr: true, wantEnd: relaycommon.StreamEndReasonClientGone,
			body: func(cancel context.CancelFunc) (io.ReadCloser, func()) {
				reader, writer := io.Pipe()
				return &noEventCancelReader{reader: reader, cancel: cancel}, func() { _ = writer.Close() }
			},
		},
		{
			name: "idle timeout before output", wantErr: true, wantEnd: relaycommon.StreamEndReasonTimeout,
			body: func(context.CancelFunc) (io.ReadCloser, func()) {
				reader, writer := io.Pipe()
				return reader, func() { _ = writer.Close() }
			},
		},
		{
			name: "message start then EOF keeps upstream usage", stream: messageStart,
			wantEnd: relaycommon.StreamEndReasonEOF, wantPrompt: 1166, wantOutput: 1, wantCacheMake: 707431,
		},
		{
			name: "complete stream keeps upstream usage", stream: fullStream,
			wantEnd: relaycommon.StreamEndReasonEOF, wantPrompt: 1166, wantOutput: 12, wantCacheMake: 707431,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			requestCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(requestCtx)

			info := &relaycommon.RelayInfo{
				OriginModelName: "claude-fable-5-1", RelayFormat: types.RelayFormatClaude,
				IsStream: true, DisablePing: true, StartTime: time.Now(),
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-fable-5-1"},
			}
			info.SetEstimatePromptTokens(testEstimatedPromptTokens)

			var body io.ReadCloser = io.NopCloser(strings.NewReader(tc.stream))
			if tc.body != nil {
				var release func()
				body, release = tc.body(cancel)
				defer release()
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       body,
			}

			usage, apiErr := ClaudeStreamHandler(c, resp, info)
			assert.Equal(t, tc.wantEnd, info.StreamStatus.EndReason)

			if tc.wantErr {
				require.NotNil(t, apiErr, "a stream without upstream events must not settle as success")
				assert.Nil(t, usage, "no usage may be produced, so the estimate can never be billed")
				assert.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
				assert.True(t, types.IsSkipRetryError(apiErr), "an abandoned request must not be replayed upstream")
				return
			}

			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, tc.wantPrompt, usage.PromptTokens)
			assert.NotEqual(t, testEstimatedPromptTokens, usage.PromptTokens, "real upstream input must win over the local estimate")
			assert.Equal(t, tc.wantOutput, usage.CompletionTokens)
			assert.Equal(t, tc.wantCacheMake, usage.PromptTokensDetails.CachedCreationTokens)
		})
	}
}

func TestIsClaudeHeartbeatEvent(t *testing.T) {
	assert.True(t, isClaudeHeartbeatEvent(`{"type":"ping"}`))
	assert.False(t, isClaudeHeartbeatEvent(`{"type":"message_start","message":{}}`))
	assert.False(t, isClaudeHeartbeatEvent(`{"type":"error","error":{"type":"overloaded_error"}}`))
	assert.False(t, isClaudeHeartbeatEvent(`not json`))
}
