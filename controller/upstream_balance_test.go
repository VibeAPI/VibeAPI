package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveUpstreamBalanceSetting(t *testing.T) *operation_setting.UpstreamBalanceSetting {
	t.Helper()
	setting := operation_setting.GetUpstreamBalanceSetting()
	previous := *setting
	previous.Accounts = append([]operation_setting.UpstreamBalanceAccount(nil), setting.Accounts...)
	t.Cleanup(func() {
		*setting = previous
	})
	return setting
}

func TestGetUpstreamBalanceSettingsNeverReturnsStoredTokens(t *testing.T) {
	setting := preserveUpstreamBalanceSetting(t)
	setting.Accounts = []operation_setting.UpstreamBalanceAccount{{
		Id: "primary", Name: "Primary", BaseURL: "https://upstream.example", Token: "secret-token",
	}}
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/option/upstream-balances", nil, 1)

	GetUpstreamBalanceSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	var data upstreamBalanceSettingsResponse
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Len(t, data.Accounts, 1)
	assert.Empty(t, data.Accounts[0].Token)
	assert.True(t, data.Accounts[0].HasToken)
	assert.NotContains(t, recorder.Body.String(), "secret-token")
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestSaveUpstreamBalanceSettingsRequiresNewTokenForChangedURL(t *testing.T) {
	setting := preserveUpstreamBalanceSetting(t)
	setting.Accounts = []operation_setting.UpstreamBalanceAccount{{
		Id: "primary", Name: "Primary", BaseURL: "https://upstream.example", Token: "secret-token",
	}}
	body := upstreamBalanceSaveRequest{
		Enabled:                true,
		Visibility:             operation_setting.UpstreamBalanceVisibilityAdmin,
		RefreshIntervalSeconds: 10,
		Accounts: []upstreamBalanceAccountInput{{
			Id: "primary", Name: "Primary", BaseURL: "https://replacement.example", HasToken: true,
		}},
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/upstream-balances", body, 1)

	SaveUpstreamBalanceSettings(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "new token is required")
	assert.NotContains(t, recorder.Body.String(), "secret-token")
}

func TestGetUpstreamBalancesEnforcesVisibility(t *testing.T) {
	setting := preserveUpstreamBalanceSetting(t)
	setting.Enabled = true
	setting.Accounts = nil

	tests := []struct {
		name       string
		visibility string
		role       int
		status     int
	}{
		{name: "all users", visibility: operation_setting.UpstreamBalanceVisibilityAll, role: common.RoleCommonUser, status: http.StatusOK},
		{name: "admin only denies user", visibility: operation_setting.UpstreamBalanceVisibilityAdmin, role: common.RoleCommonUser, status: http.StatusForbidden},
		{name: "admin only permits admin", visibility: operation_setting.UpstreamBalanceVisibilityAdmin, role: common.RoleAdminUser, status: http.StatusOK},
		{name: "unknown fails closed", visibility: "unexpected", role: common.RoleCommonUser, status: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setting.Visibility = test.visibility
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/upstream-balances/", nil, 1)
			ctx.Set("role", test.role)
			GetUpstreamBalances(ctx)
			assert.Equal(t, test.status, recorder.Code)
		})
	}
}

func TestGetUpstreamBalancesDoesNotExposeCredentials(t *testing.T) {
	setting := preserveUpstreamBalanceSetting(t)
	setting.Enabled = true
	setting.Visibility = operation_setting.UpstreamBalanceVisibilityAll
	setting.Accounts = []operation_setting.UpstreamBalanceAccount{{
		Id: "primary", Name: "Primary", BaseURL: "https://upstream.example", Token: "secret-token",
	}}
	cacheKey := common.GenerateHMAC("primary\x00Primary\x00https://upstream.example\x00secret-token")
	upstreamBalanceCacheMu.Lock()
	previousCache := upstreamBalanceCache
	upstreamBalanceCache = map[string]upstreamBalanceCacheItem{
		cacheKey: {
			result: upstreamAccountBalance{
				Id: "primary", Name: "Primary", Success: true, Balance: 12.5, Currency: "USD",
			},
			expiresAt: time.Now().Add(time.Minute),
		},
	}
	upstreamBalanceCacheMu.Unlock()
	t.Cleanup(func() {
		upstreamBalanceCacheMu.Lock()
		upstreamBalanceCache = previousCache
		upstreamBalanceCacheMu.Unlock()
	})
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/upstream-balances/", nil, 1)
	ctx.Set("role", common.RoleCommonUser)

	GetUpstreamBalances(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "12.5")
	assert.NotContains(t, recorder.Body.String(), "upstream.example")
	assert.NotContains(t, recorder.Body.String(), "secret-token")
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestGetOptionsOmitsUpstreamBalanceAccounts(t *testing.T) {
	const key = "upstream_balance_setting.accounts"
	const secret = "generic-option-secret"
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[key]
	common.OptionMap[key] = `[{"token":"` + secret + `"}]`
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap[key] = previous
		} else {
			delete(common.OptionMap, key)
		}
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/option/", nil, 1)
	GetOptions(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), key)
	assert.NotContains(t, recorder.Body.String(), secret)
}

func TestUpdateOptionRejectsUpstreamBalanceSettings(t *testing.T) {
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
		"key":   "upstream_balance_setting.accounts",
		"value": `[{"token":"should-not-be-persisted"}]`,
	}, 1)

	UpdateOption(ctx)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.NotContains(t, common.OptionMap["upstream_balance_setting.accounts"], "should-not-be-persisted")
}
