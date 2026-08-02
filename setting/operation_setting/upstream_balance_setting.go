package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	UpstreamBalanceVisibilityAll   = "all"
	UpstreamBalanceVisibilityAdmin = "admin"
)

type UpstreamBalanceAccount struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
}

type UpstreamBalanceSetting struct {
	Enabled                bool                     `json:"enabled"`
	Visibility             string                   `json:"visibility"`
	RefreshIntervalSeconds int                      `json:"refresh_interval_seconds"`
	Accounts               []UpstreamBalanceAccount `json:"accounts"`
}

var upstreamBalanceSetting = UpstreamBalanceSetting{
	Enabled:                false,
	Visibility:             UpstreamBalanceVisibilityAdmin,
	RefreshIntervalSeconds: 10,
	Accounts:               []UpstreamBalanceAccount{},
}

func init() {
	config.GlobalConfig.Register("upstream_balance_setting", &upstreamBalanceSetting)
}

func GetUpstreamBalanceSetting() *UpstreamBalanceSetting {
	return &upstreamBalanceSetting
}
