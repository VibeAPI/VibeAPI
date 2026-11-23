package relay

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/requestrule"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// checkRequestRules answers a request that matches an enabled request rule
// with that rule's error when the selected channel has request rules turned
// on. A match is the caller's error: it is not relayed, retried on another
// channel, or counted against channel health, and its log records the rule.
func checkRequestRules(c *gin.Context, info *relaycommon.RelayInfo, endpoint string) *types.NewAPIError {
	if !info.ChannelSetting.RequestRulesEnabled {
		return nil
	}
	rules := operation_setting.GetRequestRuleSetting().CompiledRules()
	if !rules.AppliesTo(endpoint) {
		return nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil
	}
	match := rules.Match(requestrule.Input{
		Endpoint:    endpoint,
		Model:       info.OriginModelName,
		Path:        c.Request.URL.Path,
		PassThrough: model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled,
		Body:        body,
		Header:      c.Request.Header,
	})
	if match == nil {
		return nil
	}
	common.SetContextKey(c, constant.ContextKeyRequestRuleMatch, match)
	info.PerformanceBusinessRejection = true
	return match.APIError()
}
