package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExportTopupLogsReturnsActualPaymentCopyText(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	originalDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
	})

	createdAt := int64(1_700_000_000)
	require.NoError(t, db.Create(&model.Log{
		UserId:    1,
		Username:  "alice",
		CreatedAt: createdAt,
		Type:      model.LogTypeTopup,
		Quota:     50_000_000,
		Content:   "使用在线充值成功，充值金额: $100.000000 额度，支付金额：80.000000",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/log/topup/export?start_timestamp=1699999999&end_timestamp=1700000001",
		nil,
	)

	ExportTopupLogs(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	expectedDate := time.Unix(createdAt, 0).In(time.Local).Format("2006/01/02")
	require.Equal(t, fmt.Sprintf("alice $80 %s\n", expectedDate), recorder.Body.String())
}
