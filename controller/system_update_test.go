package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemUpdateReadinessFailsClosedWithoutEnvironmentToken(t *testing.T) {
	t.Setenv("SYSTEM_UPDATE_READINESS_TOKEN", "")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/system-update/readiness", nil)

	GetSystemUpdateReadiness(context)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "unauthorized")
}
