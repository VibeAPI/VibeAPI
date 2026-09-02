package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type logStatResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Quota         int     `json:"quota"`
		PaymentAmount float64 `json:"payment_amount"`
		Rpm           int     `json:"rpm"`
		Tpm           int     `json:"tpm"`
	} `json:"data"`
}

func TestGetLogsStatReturnsActualTopupPaymentAmount(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	require.NoError(t, db.Create(&model.Log{
		UserId:    1,
		Username:  "alice",
		CreatedAt: 1_700_000_000,
		Type:      model.LogTypeTopup,
		Quota:     5_000_000,
		Content:   "使用在线充值成功，充值金额: $10.000000 额度，支付金额：8.000000",
		Other:     `{"payment_amount":8}`,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    1,
		Username:  "alice",
		CreatedAt: 1_700_000_001,
		Type:      model.LogTypeTopup,
		Quota:     5_000_000,
		Content:   "通过兑换码充值 $10",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/log/stat?type=1&start_timestamp=1699999999&end_timestamp=1700000002",
		nil,
	)

	GetLogsStat(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response logStatResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, 8.0, response.Data.PaymentAmount)
	assert.Zero(t, response.Data.Quota)
	assert.Zero(t, response.Data.Rpm)
	assert.Zero(t, response.Data.Tpm)
}

func TestExportTopupLogsReturnsActualPaymentCopyText(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	originalDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
	})

	users := []*model.User{
		{Username: "alice", Remark: "fcy", AffCode: "alice-topup-export"},
		{Username: "bob", Remark: "   ", AffCode: "bob-topup-export"},
	}
	require.NoError(t, db.Create(&users).Error)

	createdAt := int64(1_700_000_000)
	require.NoError(t, db.Create(&model.Log{
		UserId:    users[0].Id,
		Username:  users[0].Username,
		CreatedAt: createdAt,
		Type:      model.LogTypeTopup,
		Quota:     50_000_000,
		Content:   "使用在线充值成功，充值金额: $100.000000 额度，支付金额：80.000000",
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    users[1].Id,
		Username:  users[1].Username,
		CreatedAt: createdAt + 1,
		Type:      model.LogTypeTopup,
		Quota:     500_000,
		Content:   "使用在线充值成功，充值金额: $1.000000 额度，支付金额：1.000000",
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
	firstDate := time.Unix(createdAt, 0).In(time.Local).Format("2006/01/02")
	secondDate := time.Unix(createdAt+1, 0).In(time.Local).Format("2006/01/02")
	require.Equal(
		t,
		fmt.Sprintf("alice\t$80\t%s\tfcy\nbob\t$1\t%s\t自有用户\n", firstDate, secondDate),
		recorder.Body.String(),
	)
}
