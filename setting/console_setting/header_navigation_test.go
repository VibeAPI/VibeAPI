package console_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateHeaderNavModules(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		valid bool
	}{
		{"empty defaults", "", true},
		{"legacy navigation", `{"home":false,"pricing":{"enabled":true,"requireAuth":true}}`, true},
		{"three links with legacy icon data and disabled draft", `{"sponsors":[{"enabled":true,"name":"Docs","url":"https://docs.example.com/?lang=zh#intro","icon":"https://example.com/icon.svg"},{"enabled":false,"name":"Draft"},{"enabled":true,"name":"Tools","url":"http://tools.example.com"}]}`, true},
		{"malformed JSON", `{`, false},
		{"too many slots", `{"sponsors":[{},{},{},{}]}`, false},
		{"incorrect field type", `{"sponsors":[{"enabled":"false"}]}`, false},
		{"enabled without name", `{"sponsors":[{"enabled":true,"name":"  ","url":"https://example.com"}]}`, false},
		{"enabled without url", `{"sponsors":[{"enabled":true,"name":"Docs"}]}`, false},
		{"script link", `{"sponsors":[{"url":"javascript:alert(1)"}]}`, false},
		{"legacy icon is ignored", `{"sponsors":[{"icon":"data:image/svg+xml,test"}]}`, true},
		{"relative link", `{"sponsors":[{"url":"//example.com"}]}`, false},
		{"missing host", `{"sponsors":[{"url":"https:///path"}]}`, false},
		{"oversized name", `{"sponsors":[{"name":"` + strings.Repeat("a", 81) + `"}]}`, false},
		{"oversized url", `{"sponsors":[{"url":"https://example.com/` + strings.Repeat("a", 2048) + `"}]}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHeaderNavModules(tt.raw)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
