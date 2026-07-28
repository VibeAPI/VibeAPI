package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateUserPersistsManualInviter(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	inviter := &model.User{Username: "controller-inviter", AffCode: "controller-inviter-code", Role: common.RoleCommonUser}
	invitee := &model.User{Username: "controller-invitee", DisplayName: "Controller Invitee", AffCode: "controller-invitee-code", Group: "default", Role: common.RoleCommonUser}
	require.NoError(t, db.Create(inviter).Error)
	require.NoError(t, db.Create(invitee).Error)

	body := strings.NewReader(`{"id":` + strconv.Itoa(invitee.Id) + `,"username":"controller-invitee","display_name":"Controller Invitee","group":"default","inviter_id":` + strconv.Itoa(inviter.Id) + `}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/user/", body)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 999)
	ctx.Set("role", common.RoleAdminUser)

	UpdateUser(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, invitee.Id).Error)
	assert.Equal(t, inviter.Id, updated.InviterId)
}
