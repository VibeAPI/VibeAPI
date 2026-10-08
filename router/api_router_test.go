package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTopupExportRouteRequiresAdmin(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuditLog{}))

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousRateLimit := common.RedisEnabled, common.GlobalApiRateLimitEnable
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled, common.GlobalApiRateLimitEnable = false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled, common.GlobalApiRateLimitEnable = previousRedis, previousRateLimit
	})

	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser} {
		username, token := "user", "topup-export-user-token"
		if role == common.RoleAdminUser {
			username, token = "admin", "topup-export-admin-token"
		}
		require.NoError(t, db.Create(&model.User{
			Username: username, Role: role, Status: common.UserStatusEnabled,
			AccessToken: &token, AffCode: username, Group: "default",
		}).Error)
	}

	engine := gin.New()
	SetApiRouter(engine)
	for _, tc := range []struct {
		name   string
		token  string
		status int
		body   string
	}{
		{name: "anonymous", status: http.StatusUnauthorized},
		{name: "ordinary user", token: "topup-export-user-token", status: http.StatusForbidden, body: "AUTH_INSUFFICIENT_PRIVILEGE"},
		// An invalid range proves the real export handler was reached without querying logs.
		{name: "admin", token: "topup-export-admin-token", status: http.StatusBadRequest, body: "invalid time range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/log/topup/export", nil)
			if tc.token != "" {
				request.Header.Set("Authorization", "Bearer "+tc.token)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			if tc.body != "" {
				assert.Contains(t, response.Body.String(), tc.body)
			}
		})
	}
}
