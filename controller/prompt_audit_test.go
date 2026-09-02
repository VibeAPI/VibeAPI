package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestPromptAuditSettingsResponseSerializesNilListsAsEmptyArrays(t *testing.T) {
	response := promptAuditSettingsResponse(operation_setting.PromptAuditSetting{})

	payload, err := common.Marshal(response)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"protected_channel_ids":[]`)
	require.Contains(t, string(payload), `"audience_user_ids":[]`)
}

func TestPromptAuditSettingsResponseDataSerializesNilRestrictedUsersAsEmptyArray(t *testing.T) {
	response := promptAuditSettingsResponseData{
		promptAuditSettingsInput: promptAuditSettingsResponse(operation_setting.PromptAuditSetting{}),
		RestrictedUsers:          []model.ChannelBlacklistUser{},
	}

	payload, err := common.Marshal(response)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"restricted_users":[]`)
}
