package controller

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/systemupdate"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetSystemUpdateCapabilities(c *gin.Context) {
	common.ApiSuccess(c, service.GetSystemUpdateService().Capabilities(c.Request.Context()))
}

func ListSystemUpdateReleases(c *gin.Context) {
	result, err := service.GetSystemUpdateService().Releases(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func PreflightSystemUpdate(c *gin.Context) {
	var request systemupdate.PreflightRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	result, err := service.GetSystemUpdateService().Preflight(c.Request.Context(), request)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func StartSystemUpdateOperation(c *gin.Context) {
	var request systemupdate.StartOperationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	result, err := service.GetSystemUpdateService().StartOperation(c.Request.Context(), request)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAudit(c, "system_update.start", map[string]interface{}{
		"operation_id": result.ID,
		"release_id":   result.ReleaseID,
		"version":      result.Version,
		"digest":       result.Digest,
	})
	common.ApiSuccess(c, result)
}

func ListSystemUpdateOperations(c *gin.Context) {
	result, err := service.GetSystemUpdateService().ListOperations(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetSystemUpdateOperation(c *gin.Context) {
	result, err := service.GetSystemUpdateService().GetOperation(c.Request.Context(), c.Param("operation_id"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

// GetSystemUpdateReadiness is intentionally outside session authentication: a
// replacement container is probed by the local updater before traffic is
// switched. Access is protected by a dedicated environment-only bearer token.
func GetSystemUpdateReadiness(c *gin.Context) {
	expected := strings.TrimSpace(os.Getenv("SYSTEM_UPDATE_READINESS_TOKEN"))
	presented := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if expected == "" || presented == "" || len(expected) != len(presented) || subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "unauthorized"})
		return
	}
	if err := model.PingDB(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "database is unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": systemupdate.Readiness{Ready: true, Version: common.Version}})
}
