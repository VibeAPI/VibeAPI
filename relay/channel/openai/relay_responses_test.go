package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesHandlerReturnsRequestedModelNameWithoutDroppingFields(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_1","model":"gpt-upstream-versioned","output":[],"usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5},"vendor_extension":{"trace_id":"trace-1"}}`,
		)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-public",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-upstream",
			ChannelSetting: dto.ChannelSettings{
				ReturnRequestModelName: true,
			},
		},
	}

	usage, err := OaiResponsesHandler(c, info, resp)
	require.Nil(t, err)
	require.Equal(t, 5, usage.TotalTokens)
	require.JSONEq(t, `{"id":"resp_1","model":"gpt-public","output":[],"usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5},"vendor_extension":{"trace_id":"trace-1"}}`, recorder.Body.String())
}

func TestOaiResponsesStreamHandlerReturnsRequestedModelNameInEveryResponseEvent(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-upstream-versioned","output":[]}}`,
		`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-upstream-versioned","output":[],"usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}}`,
		`data: [DONE]`,
		``,
	}, "\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-public",
		DisablePing:     true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-upstream",
			ChannelSetting: dto.ChannelSettings{
				ReturnRequestModelName: true,
			},
		},
	}

	usage, err := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.Equal(t, 5, usage.TotalTokens)
	require.Equal(t, 2, strings.Count(recorder.Body.String(), `"model":"gpt-public"`))
	require.NotContains(t, recorder.Body.String(), `gpt-upstream-versioned`)
}
