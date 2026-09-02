package common

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponseModelNameUsesRequestModelWhenEnabled(t *testing.T) {
	info := &RelayInfo{
		OriginModelName: "gpt-public",
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "gpt-upstream",
			ChannelSetting: dto.ChannelSettings{
				ReturnRequestModelName: true,
			},
		},
	}

	assert.Equal(t, "gpt-public", info.ResponseModelName())
}

func TestResponseModelNameDefaultsToUpstreamModel(t *testing.T) {
	info := &RelayInfo{
		OriginModelName: "gpt-public",
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "gpt-upstream",
		},
	}

	assert.Equal(t, "gpt-upstream", info.ResponseModelName())
}

func TestExposeResponseModelNamePreservesProviderModelWhenDisabled(t *testing.T) {
	info := &RelayInfo{
		OriginModelName: "gpt-public",
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "gpt-upstream",
		},
	}

	assert.Equal(t, "gpt-upstream-versioned", info.ExposeResponseModelName("gpt-upstream-versioned"))
}

func TestExposeResponseModelNameHandlesMissingChannelMetadata(t *testing.T) {
	info := &RelayInfo{OriginModelName: "gpt-public"}

	assert.Equal(t, "gpt-upstream-versioned", info.ExposeResponseModelName("gpt-upstream-versioned"))
}

func TestResponseModelNameOrFallsBackToProviderModel(t *testing.T) {
	info := &RelayInfo{ChannelMeta: &ChannelMeta{}}

	assert.Equal(t, "gpt-provider", info.ResponseModelNameOr("gpt-provider"))
}

func TestRewriteResponseModelOnlyRewritesExistingPaths(t *testing.T) {
	info := &RelayInfo{
		OriginModelName: "gpt-public",
		ChannelMeta: &ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				ReturnRequestModelName: true,
			},
		},
	}
	input := []byte(`{"model":"gpt-upstream","vendor":{"trace_id":"trace-1"}}`)

	output, err := RewriteResponseModel(input, info, "model", "response.model")
	require.NoError(t, err)
	assert.Equal(t, "gpt-public", gjson.GetBytes(output, "model").String())
	assert.Equal(t, "trace-1", gjson.GetBytes(output, "vendor.trace_id").String())
	assert.False(t, gjson.GetBytes(output, "response").Exists())
}
