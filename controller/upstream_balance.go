package controller

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
)

const (
	upstreamBalancePath          = "/api/usage/token/balance"
	maxUpstreamBalanceAccounts   = 20
	maxUpstreamBalanceBodySize   = 64 << 10
	maxUpstreamSettingsBodySize  = 256 << 10
	maxUpstreamAccountIdLength   = 128
	maxUpstreamAccountNameLength = 128
	maxUpstreamBaseURLLength     = 2048
	maxUpstreamTokenLength       = 128
	minUpstreamRefreshSeconds    = 5
	maxUpstreamRefreshSeconds    = 300
	upstreamBalanceFailureTTL    = 5
)

type upstreamBalanceAccountInput struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Token    string `json:"token"`
	HasToken bool   `json:"has_token"`
}

type upstreamBalanceSettingsResponse struct {
	Enabled                bool                          `json:"enabled"`
	Visibility             string                        `json:"visibility"`
	RefreshIntervalSeconds int                           `json:"refresh_interval_seconds"`
	Accounts               []upstreamBalanceAccountInput `json:"accounts"`
}

type upstreamBalanceSaveRequest struct {
	Enabled                bool                          `json:"enabled"`
	Visibility             string                        `json:"visibility"`
	RefreshIntervalSeconds int                           `json:"refresh_interval_seconds"`
	Accounts               []upstreamBalanceAccountInput `json:"accounts"`
}

type upstreamAccountBalance struct {
	Id           string  `json:"id"`
	Name         string  `json:"name"`
	Success      bool    `json:"success"`
	Balance      float64 `json:"balance"`
	Currency     string  `json:"currency,omitempty"`
	Quota        int     `json:"quota"`
	QuotaPerUnit float64 `json:"quota_per_unit"`
	UpdatedAt    int64   `json:"updated_at,omitempty"`
	Message      string  `json:"message,omitempty"`
}

type upstreamBalanceAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Quota        int     `json:"quota"`
		QuotaPerUnit float64 `json:"quota_per_unit"`
		Balance      float64 `json:"balance"`
		Currency     string  `json:"currency"`
		UpdatedAt    int64   `json:"updated_at"`
	} `json:"data"`
}

type upstreamBalanceCacheItem struct {
	result    upstreamAccountBalance
	expiresAt time.Time
}

var (
	upstreamBalanceCacheMu sync.Mutex
	upstreamBalanceCache   = make(map[string]upstreamBalanceCacheItem)
	upstreamBalanceGroup   singleflight.Group
)

func GetUpstreamBalanceSettings(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	setting := operation_setting.GetUpstreamBalanceSetting()
	accounts := make([]upstreamBalanceAccountInput, 0, len(setting.Accounts))
	for _, account := range setting.Accounts {
		accounts = append(accounts, upstreamBalanceAccountInput{
			Id:       account.Id,
			Name:     account.Name,
			BaseURL:  account.BaseURL,
			Token:    "",
			HasToken: account.Token != "",
		})
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, upstreamBalanceSettingsResponse{
		Enabled:                setting.Enabled,
		Visibility:             setting.Visibility,
		RefreshIntervalSeconds: setting.RefreshIntervalSeconds,
		Accounts:               accounts,
	})
}

func SaveUpstreamBalanceSettings(c *gin.Context) {
	var req upstreamBalanceSaveRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUpstreamSettingsBodySize)
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid parameters"})
		return
	}
	if req.Visibility != operation_setting.UpstreamBalanceVisibilityAll &&
		req.Visibility != operation_setting.UpstreamBalanceVisibilityAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid visibility"})
		return
	}
	if req.RefreshIntervalSeconds < minUpstreamRefreshSeconds || req.RefreshIntervalSeconds > maxUpstreamRefreshSeconds {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("Refresh interval must be between %d and %d seconds", minUpstreamRefreshSeconds, maxUpstreamRefreshSeconds)})
		return
	}
	if len(req.Accounts) > maxUpstreamBalanceAccounts {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("A maximum of %d upstream accounts is allowed", maxUpstreamBalanceAccounts)})
		return
	}
	common.OptionMapRWMutex.RLock()
	currentSetting := operation_setting.GetUpstreamBalanceSetting()
	existingById := make(map[string]operation_setting.UpstreamBalanceAccount)
	for _, account := range currentSetting.Accounts {
		existingById[account.Id] = account
	}
	common.OptionMapRWMutex.RUnlock()
	accounts := make([]operation_setting.UpstreamBalanceAccount, 0, len(req.Accounts))
	seen := make(map[string]struct{}, len(req.Accounts))
	for _, input := range req.Accounts {
		input.Id = strings.TrimSpace(input.Id)
		input.Name = strings.TrimSpace(input.Name)
		input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
		input.Token = strings.TrimSpace(input.Token)
		if input.Id == "" || input.Name == "" || input.BaseURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Account ID, name, and URL are required"})
			return
		}
		if len(input.Id) > maxUpstreamAccountIdLength || len(input.Name) > maxUpstreamAccountNameLength ||
			len(input.BaseURL) > maxUpstreamBaseURLLength {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Upstream account fields exceed the allowed length"})
			return
		}
		if _, ok := seen[input.Id]; ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Upstream account IDs must be unique"})
			return
		}
		seen[input.Id] = struct{}{}
		existing, exists := existingById[input.Id]
		urlChanged := exists && input.BaseURL != strings.TrimRight(strings.TrimSpace(existing.BaseURL), "/")
		if req.Enabled && input.Token == "" && urlChanged {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "A new token is required when changing the upstream URL"})
			return
		}
		parsedURL, err := url.ParseRequestURI(input.BaseURL)
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid upstream URL"})
			return
		}
		// A disabled module does not make outbound requests. Allow operators to
		// save/clean up a previously valid configuration even if its host is no
		// longer reachable or is now rejected by the current SSRF policy. The
		// same validation is enforced before the module can be enabled.
		if req.Enabled {
			if err := service.ValidateSSRFProtectedFetchURL(input.BaseURL); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Upstream URL is blocked by the current fetch security policy"})
				return
			}
		}

		if input.Token == "" && exists && !urlChanged {
			input.Token = existing.Token
		}
		input.Token = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(input.Token, "Bearer "), "bearer "))
		input.Token = strings.TrimPrefix(input.Token, "sk-")
		if req.Enabled && input.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "A token is required for every upstream account"})
			return
		}
		if len(input.Token) > maxUpstreamTokenLength {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Upstream token exceeds the allowed length"})
			return
		}
		accounts = append(accounts, operation_setting.UpstreamBalanceAccount{
			Id:      input.Id,
			Name:    input.Name,
			BaseURL: input.BaseURL,
			Token:   input.Token,
		})
	}
	if req.Enabled && len(accounts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "At least one upstream account is required when the module is enabled"})
		return
	}

	settingsJSON, err := common.Marshal(accounts)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		"upstream_balance_setting.enabled":                  fmt.Sprintf("%t", req.Enabled),
		"upstream_balance_setting.visibility":               req.Visibility,
		"upstream_balance_setting.refresh_interval_seconds": fmt.Sprintf("%d", req.RefreshIntervalSeconds),
		"upstream_balance_setting.accounts":                 string(settingsJSON),
	}); err != nil {
		common.ApiError(c, err)
		return
	}

	upstreamBalanceCacheMu.Lock()
	upstreamBalanceCache = make(map[string]upstreamBalanceCacheItem)
	upstreamBalanceCacheMu.Unlock()
	recordManageAudit(c, "upstream_balance.update", map[string]interface{}{"account_count": len(accounts)})
	common.ApiSuccess(c, nil)
}

func GetUpstreamBalances(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	setting := operation_setting.GetUpstreamBalanceSetting()
	if !setting.Enabled {
		common.OptionMapRWMutex.RUnlock()
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Upstream balance module is disabled"})
		return
	}
	role := c.GetInt("role")
	if setting.Visibility != operation_setting.UpstreamBalanceVisibilityAll && role < common.RoleAdminUser {
		common.OptionMapRWMutex.RUnlock()
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Insufficient privilege"})
		return
	}

	accounts := append([]operation_setting.UpstreamBalanceAccount(nil), setting.Accounts...)
	refreshIntervalSeconds := setting.RefreshIntervalSeconds
	common.OptionMapRWMutex.RUnlock()
	if len(accounts) > maxUpstreamBalanceAccounts {
		accounts = accounts[:maxUpstreamBalanceAccounts]
	}
	results := make([]upstreamAccountBalance, len(accounts))
	var wg sync.WaitGroup
	for index, account := range accounts {
		wg.Add(1)
		go func(index int, account operation_setting.UpstreamBalanceAccount) {
			defer wg.Done()
			results[index] = getCachedUpstreamBalance(c.Request.Context(), account, refreshIntervalSeconds)
		}(index, account)
	}
	wg.Wait()
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, results)
}

func getCachedUpstreamBalance(ctx context.Context, account operation_setting.UpstreamBalanceAccount, ttlSeconds int) upstreamAccountBalance {
	if ttlSeconds < minUpstreamRefreshSeconds {
		ttlSeconds = minUpstreamRefreshSeconds
	} else if ttlSeconds > maxUpstreamRefreshSeconds {
		ttlSeconds = maxUpstreamRefreshSeconds
	}
	cacheKey := common.GenerateHMAC(strings.Join([]string{account.Id, account.Name, account.BaseURL, account.Token}, "\x00"))
	now := time.Now()
	upstreamBalanceCacheMu.Lock()
	item, ok := upstreamBalanceCache[cacheKey]
	upstreamBalanceCacheMu.Unlock()
	if ok && now.Before(item.expiresAt) {
		return item.result
	}

	value, _, _ := upstreamBalanceGroup.Do(cacheKey, func() (interface{}, error) {
		upstreamBalanceCacheMu.Lock()
		item, ok := upstreamBalanceCache[cacheKey]
		upstreamBalanceCacheMu.Unlock()
		if ok && time.Now().Before(item.expiresAt) {
			return item.result, nil
		}

		result := fetchUpstreamBalance(context.WithoutCancel(ctx), account)
		upstreamBalanceCacheMu.Lock()
		cacheTTL := ttlSeconds
		if !result.Success && cacheTTL > upstreamBalanceFailureTTL {
			cacheTTL = upstreamBalanceFailureTTL
		}
		upstreamBalanceCache[cacheKey] = upstreamBalanceCacheItem{
			result:    result,
			expiresAt: time.Now().Add(time.Duration(cacheTTL) * time.Second),
		}
		upstreamBalanceCacheMu.Unlock()
		return result, nil
	})
	return value.(upstreamAccountBalance)
}

func fetchUpstreamBalance(ctx context.Context, account operation_setting.UpstreamBalanceAccount) upstreamAccountBalance {
	result := upstreamAccountBalance{Id: account.Id, Name: account.Name}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	requestURL := strings.TrimRight(account.BaseURL, "/") + upstreamBalancePath
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		result.Message = "Failed to create upstream request"
		return result
	}
	request.Header.Set("Authorization", "Bearer sk-"+account.Token)
	request.Header.Set("Accept", "application/json")

	client := service.GetSSRFProtectedHTTPClient()
	balanceClient := *client
	balanceClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := balanceClient.Do(request)
	if err != nil {
		result.Message = "Upstream request failed"
		return result
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxUpstreamBalanceBodySize))
	if err != nil {
		result.Message = "Failed to read upstream response"
		return result
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		result.Message = fmt.Sprintf("Upstream returned HTTP %d", response.StatusCode)
		return result
	}

	var payload upstreamBalanceAPIResponse
	if err := common.Unmarshal(body, &payload); err != nil || !payload.Success {
		result.Message = "Invalid upstream balance response"
		return result
	}
	if payload.Data.Quota < 0 {
		result.Message = "Invalid upstream quota"
		return result
	}
	if payload.Data.QuotaPerUnit <= 0 || math.IsNaN(payload.Data.QuotaPerUnit) || math.IsInf(payload.Data.QuotaPerUnit, 0) {
		result.Message = "Invalid upstream quota unit"
		return result
	}
	balance := float64(payload.Data.Quota) / payload.Data.QuotaPerUnit
	if math.IsNaN(balance) || math.IsInf(balance, 0) {
		result.Message = "Invalid upstream balance"
		return result
	}
	result.Success = true
	result.Balance = balance
	result.Currency = "USD"
	result.Quota = payload.Data.Quota
	result.QuotaPerUnit = payload.Data.QuotaPerUnit
	result.UpdatedAt = payload.Data.UpdatedAt
	return result
}
