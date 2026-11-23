package operation_setting

import (
	"github.com/QuantumNous/new-api/pkg/requestrule"
	"github.com/QuantumNous/new-api/setting/config"
)

// RequestRuleSetting holds the request rules that channels with request rules
// turned on check, in order, before relaying. The first rule a request
// matches answers it with the rule's own error instead of relaying it.
type RequestRuleSetting struct {
	// Rules is a JSON array in the format of pkg/requestrule. The default is
	// the preset rule list.
	Rules string `json:"rules"`
}

var requestRuleSetting = RequestRuleSetting{
	Rules: requestrule.DefaultRulesJSON(),
}

func init() {
	config.GlobalConfig.Register("request_rule_setting", &requestRuleSetting)
}

func GetRequestRuleSetting() *RequestRuleSetting {
	return &requestRuleSetting
}

// CompiledRules returns the rules ready for matching. Saving validates them,
// so compiling here only reuses or rebuilds the cached list.
func (s *RequestRuleSetting) CompiledRules() *requestrule.Set {
	return requestrule.Cached(s.Rules)
}
