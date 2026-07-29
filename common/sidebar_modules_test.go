package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func withSidebarModulesAdmin(t *testing.T, raw string) {
	t.Helper()

	OptionMapRWMutex.Lock()
	if OptionMap == nil {
		OptionMap = map[string]string{}
	}
	previous, existed := OptionMap["SidebarModulesAdmin"]
	OptionMap["SidebarModulesAdmin"] = raw
	OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		OptionMapRWMutex.Lock()
		defer OptionMapRWMutex.Unlock()
		if existed {
			OptionMap["SidebarModulesAdmin"] = previous
			return
		}
		delete(OptionMap, "SidebarModulesAdmin")
	})
}

func TestIsSidebarModuleRootOnlyDefaultsToFalse(t *testing.T) {
	for _, raw := range []string{"", "not-json", `{}`} {
		t.Run(raw, func(t *testing.T) {
			withSidebarModulesAdmin(t, raw)
			assert.False(t, IsSidebarModuleRootOnly("admin", "channel"))
		})
	}
}

func TestIsSidebarModuleRootOnlySupportsPersistedBooleanFormats(t *testing.T) {
	for _, value := range []string{"true", `"true"`, `"1"`, "1"} {
		t.Run(value, func(t *testing.T) {
			withSidebarModulesAdmin(t, `{"admin":{"channelRootOnly":`+value+`}}`)
			assert.True(t, IsSidebarModuleRootOnly("admin", "channel"))
		})
	}
}

func TestCanViewSidebarModuleRestrictsOnlyNonRootRoles(t *testing.T) {
	withSidebarModulesAdmin(t, `{"admin":{"channelRootOnly":true}}`)

	assert.False(t, CanViewSidebarModule(RoleAdminUser, "admin", "channel"))
	assert.True(t, CanViewSidebarModule(RoleRootUser, "admin", "channel"))
	assert.True(t, CanViewSidebarModule(RoleAdminUser, "admin", "setting"))
}
