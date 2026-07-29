package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func withSidebarModulesAdmin(t *testing.T, raw string) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	previous, existed := common.OptionMap["SidebarModulesAdmin"]
	common.OptionMap["SidebarModulesAdmin"] = raw
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap["SidebarModulesAdmin"] = previous
			return
		}
		delete(common.OptionMap, "SidebarModulesAdmin")
	})
}

func performSidebarModuleRequest(role int) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Next()
	})
	router.GET(
		"/api/channel",
		RootOnlySidebarModuleAuth("admin", "channel"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/channel", nil))
	return recorder
}

func TestRootOnlySidebarModuleAuthRejectsAdmin(t *testing.T) {
	withSidebarModulesAdmin(t, `{"admin":{"channelRootOnly":true}}`)

	recorder := performSidebarModuleRequest(common.RoleAdminUser)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestRootOnlySidebarModuleAuthAllowsRoot(t *testing.T) {
	withSidebarModulesAdmin(t, `{"admin":{"channelRootOnly":true}}`)

	recorder := performSidebarModuleRequest(common.RoleRootUser)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestRootOnlySidebarModuleAuthKeepsBackwardCompatibleAdminAccess(t *testing.T) {
	withSidebarModulesAdmin(t, `{}`)

	recorder := performSidebarModuleRequest(common.RoleAdminUser)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}
