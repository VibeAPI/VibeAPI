package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTopupQuotaSupportsStructuredAndLegacyLogs(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	originalExchangeRate := operation_setting.USDExchangeRate
	originalCustomExchangeRate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.USDExchangeRate = originalExchangeRate
		operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = originalCustomExchangeRate
	})
	common.QuotaPerUnit = 500_000
	operation_setting.USDExchangeRate = 7
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 0.8

	testCases := []struct {
		name     string
		log      *Log
		expected int
		ok       bool
	}{
		{
			name:     "structured top-up quota",
			log:      &Log{Type: LogTypeTopup, Quota: 5_000_000, Content: "completed"},
			expected: 5_000_000,
			ok:       true,
		},
		{
			name:     "legacy USD amount",
			log:      &Log{Type: LogTypeTopup, Content: "使用在线充值成功，充值金额: ＄10.500000 额度"},
			expected: 5_250_000,
			ok:       true,
		},
		{
			name:     "legacy CNY amount",
			log:      &Log{Type: LogTypeTopup, Content: "使用在线充值成功，充值金额: ¥70.000000 额度"},
			expected: 5_000_000,
			ok:       true,
		},
		{
			name:     "legacy token amount",
			log:      &Log{Type: LogTypeTopup, Content: "使用在线充值成功，充值金额: 5000000 点额度"},
			expected: 5_000_000,
			ok:       true,
		},
		{
			name:     "legacy custom currency amount",
			log:      &Log{Type: LogTypeTopup, Content: "使用在线充值成功，充值金额: €8.000000 额度"},
			expected: 5_000_000,
			ok:       true,
		},
		{
			name:     "legacy Creem quota",
			log:      &Log{Type: LogTypeTopup, Content: "使用Creem充值成功，充值额度: 5000000，支付金额：10.00"},
			expected: 5_000_000,
			ok:       true,
		},
		{
			name: "pending corporate order is not a recharge",
			log:  &Log{Type: LogTypeTopup, Content: "提交对公支付订单，订单号：123，充值金额：10，支付金额：70.00"},
			ok:   false,
		},
		{
			name: "subscription purchase is not a recharge",
			log:  &Log{Type: LogTypeTopup, Content: "订阅购买成功，套餐: Pro，支付金额: 10.00"},
			ok:   false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			quota, ok := GetTopupQuota(testCase.log)
			if !testCase.ok {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, testCase.expected, quota)
		})
	}
}
