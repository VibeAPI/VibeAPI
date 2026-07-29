package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactLogChannelInfoRemovesChannelDiagnostics(t *testing.T) {
	logs := []*Log{
		{
			ChannelId:   12,
			ChannelName: "upstream-a",
			Content:     "request completed",
			Other: `{
				"admin_info": {
					"use_channel": [12, 18],
					"channel_affinity": {"rule_name": "sticky"},
					"is_multi_key": true,
					"multi_key_index": 2,
					"fallback_channel_id": 18,
					"quota_saturation": {"reason": "overflow"}
				},
				"billing_mode": "standard"
			}`,
		},
	}

	RedactLogChannelInfo(logs)

	assert.Zero(t, logs[0].ChannelId)
	assert.Empty(t, logs[0].ChannelName)
	assert.Equal(t, "request completed", logs[0].Content)
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.NotContains(t, adminInfo, "use_channel")
	assert.NotContains(t, adminInfo, "channel_affinity")
	assert.NotContains(t, adminInfo, "is_multi_key")
	assert.NotContains(t, adminInfo, "multi_key_index")
	assert.NotContains(t, adminInfo, "fallback_channel_id")
	assert.Contains(t, adminInfo, "quota_saturation")
	assert.Equal(t, "standard", other["billing_mode"])
}

func TestRedactLogChannelInfoRemovesChannelAuditPayload(t *testing.T) {
	logs := []*Log{
		{
			ChannelId: 9,
			Content:   "updated channel secret",
			Other: `{
				"op": {"action": "channel.update", "params": {"name": "secret-provider"}},
				"audit_info": {"route": "PUT /api/channel", "operator": "admin"},
				"unrelated": "keep"
			}`,
		},
	}

	RedactLogChannelInfo(logs)

	assert.Empty(t, logs[0].Content)
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, other, "op")
	assert.NotContains(t, other, "audit_info")
	assert.Equal(t, "keep", other["unrelated"])
}
