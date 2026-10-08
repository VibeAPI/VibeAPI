package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionRejectsInvalidCorporatePaymentAllowedGroups(t *testing.T) {
	previous := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previous)) })

	for _, testCase := range []struct {
		name  string
		value string
	}{
		{name: "not an array", value: `{}`},
		{name: "blank group", value: `[" "]`},
		{name: "duplicate group", value: `["vip","vip"]`},
		{name: "missing group", value: `["missing"]`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			context, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", OptionUpdateRequest{
				Key:   "payment_setting.corporate_payment_allowed_groups",
				Value: testCase.value,
			}, 1)

			UpdateOption(context)

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"success":false`)
		})
	}
}
