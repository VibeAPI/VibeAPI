package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-gonic/gin"
)

// RootOnlySidebarModuleAuth narrows an already authenticated route when the
// corresponding sidebar module is configured as root-only.
func RootOnlySidebarModuleAuth(section, module string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.CanViewSidebarModule(c.GetInt("role"), section, module) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgAuthInsufficientPrivilege),
		})
		c.Abort()
	}
}
