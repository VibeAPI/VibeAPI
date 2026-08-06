package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var transferDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func SubmitCorporateTopUp(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxCorporateProofBytes+(1<<20))
	settings := operation_setting.GetPaymentSetting()
	if !operation_setting.IsPaymentComplianceConfirmed() || !settings.CorporatePaymentEnabled ||
		settings.CorporatePaymentName == "" || settings.CorporatePaymentBank == "" ||
		settings.CorporatePaymentAccount == "" {
		common.ApiErrorMsg(c, "对公支付未启用")
		return
	}
	userID := c.GetInt("id")
	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}
	if !operation_setting.IsCorporatePaymentGroupAllowed(group) {
		common.ApiErrorMsg(c, "当前用户分组不可使用对公支付")
		return
	}

	amount, err := strconv.ParseInt(c.PostForm("amount"), 10, 64)
	if err != nil || amount < int64(settings.CorporatePaymentMinTopUp) || amount > int64(common.MaxQuota) {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", settings.CorporatePaymentMinTopUp))
		return
	}
	quotaAmount := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		if common.QuotaPerUnit <= 0 {
			common.ApiErrorMsg(c, "额度换算配置无效")
			return
		}
		quotaAmount = quotaAmount.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	if _, clamp := common.QuotaFromDecimalChecked(quotaAmount.Mul(decimal.NewFromFloat(common.QuotaPerUnit))); clamp != nil {
		common.ApiErrorMsg(c, "充值金额超出允许范围")
		return
	}

	bankName := strings.TrimSpace(c.PostForm("bank_name"))
	bankAccount := strings.TrimSpace(c.PostForm("bank_account"))
	payerName := strings.TrimSpace(c.PostForm("payer_name"))
	transferDate := strings.TrimSpace(c.PostForm("transfer_date"))
	phone := strings.TrimSpace(c.PostForm("phone"))
	if bankName == "" || bankAccount == "" || payerName == "" || phone == "" || !transferDatePattern.MatchString(transferDate) {
		common.ApiErrorMsg(c, "请完整填写汇款信息")
		return
	}
	if _, err := time.Parse("2006-01-02", transferDate); err != nil {
		common.ApiErrorMsg(c, "汇款日期无效")
		return
	}
	if len(bankName) > 255 || len(bankAccount) > 255 || len(payerName) > 255 || len(phone) > 50 {
		common.ApiErrorMsg(c, "汇款信息过长")
		return
	}

	fileHeader, err := c.FormFile("proof")
	if err != nil {
		common.ApiErrorMsg(c, "请上传转账凭证")
		return
	}
	proofData, mimeType, extension, err := readCorporateProof(fileHeader)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	payMoney := getPayMoney(amount, group)
	if math.IsNaN(payMoney) || math.IsInf(payMoney, 0) || payMoney < 0.01 {
		common.ApiErrorMsg(c, "充值金额过低")
		return
	}

	tradeNo := fmt.Sprintf("CORP%dNO%s%d", userID, common.GetRandomString(6), time.Now().Unix())
	proofStorage, err := service.NewCorporateProofStorage()
	if err != nil {
		common.SysError(fmt.Sprintf("initialize corporate proof storage: %v", err))
		common.ApiErrorMsg(c, "凭证存储配置无效，请联系管理员")
		return
	}
	proofLocation, err := proofStorage.Save(c.Request.Context(), tradeNo+extension, mimeType, proofData)
	if err != nil {
		common.SysError(fmt.Sprintf("store corporate proof: %v", err))
		common.ApiErrorMsg(c, "保存转账凭证失败，请稍后重试")
		return
	}
	topUp := &model.TopUp{
		UserId:              userID,
		Amount:              quotaAmount.IntPart(),
		Money:               payMoney,
		TradeNo:             tradeNo,
		PaymentMethod:       model.PaymentMethodCorporate,
		PaymentProvider:     model.PaymentProviderCorporate,
		CreateTime:          time.Now().Unix(),
		Status:              common.TopUpStatusPending,
		TransferBankName:    bankName,
		TransferBankAccount: bankAccount,
		TransferPayerName:   payerName,
		TransferDate:        transferDate,
		TransferPhone:       phone,
		ProofOriginalName:   filepath.Base(fileHeader.Filename),
		ProofStorageName:    proofLocation.StorageName,
		ProofURL:            proofLocation.URL,
		ProofMimeType:       mimeType,
	}
	if err := topUp.Insert(); err != nil {
		if cleanupErr := proofStorage.Delete(context.Background(), proofLocation.URL, proofLocation.StorageName); cleanupErr != nil {
			common.SysError(fmt.Sprintf("rollback corporate proof after database error: %v", cleanupErr))
		}
		common.ApiErrorMsg(c, "提交对公支付凭证失败")
		return
	}
	model.RecordTopupLog(
		userID,
		fmt.Sprintf("提交对公支付订单，订单号：%s，充值金额：%d，支付金额：%.2f", tradeNo, topUp.Amount, topUp.Money),
		c.ClientIP(),
		topUp.PaymentMethod,
		"submitted",
		0,
	)

	common.ApiSuccess(c, gin.H{"trade_no": tradeNo})
}

func GetCorporateTopUpProof(c *gin.Context) {
	topUp := model.GetTopUpByTradeNo(c.Param("trade_no"))
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderCorporate ||
		(topUp.ProofURL == "" && topUp.ProofStorageName == "") {
		c.Status(http.StatusNotFound)
		return
	}
	if c.GetInt("role") < common.RoleAdminUser && topUp.UserId != c.GetInt("id") {
		c.Status(http.StatusForbidden)
		return
	}

	proofStorage, err := service.NewCorporateProofStorage()
	if err != nil {
		common.SysError(fmt.Sprintf("initialize corporate proof storage: %v", err))
		c.Status(http.StatusNotFound)
		return
	}
	proofData, storedMimeType, err := proofStorage.Read(c.Request.Context(), topUp.ProofURL, topUp.ProofStorageName)
	if err != nil {
		common.SysError(fmt.Sprintf("read corporate proof: %v", err))
		c.Status(http.StatusNotFound)
		return
	}
	mimeType := topUp.ProofMimeType
	if mimeType == "" {
		mimeType = storedMimeType
	}
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(topUp.ProofOriginalName)))
	c.Data(http.StatusOK, mimeType, proofData)
}

func readCorporateProof(fileHeader *multipart.FileHeader) ([]byte, string, string, error) {
	if fileHeader.Size <= 0 || fileHeader.Size > service.MaxCorporateProofBytes {
		return nil, "", "", errors.New("转账凭证大小不能超过 5MB")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, "", "", errors.New("无法读取转账凭证")
	}
	defer file.Close()

	if strings.Contains(fileHeader.Filename, "\x00") {
		return nil, "", "", errors.New("转账凭证文件名无效")
	}
	buffer := make([]byte, 512)
	read, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, "", "", errors.New("无法读取转账凭证")
	}
	mimeType := http.DetectContentType(buffer[:read])
	extension := ""
	switch mimeType {
	case "image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	case "application/pdf":
		if read < 5 || string(buffer[:5]) != "%PDF-" {
			return nil, "", "", errors.New("PDF 转账凭证格式无效")
		}
		extension = ".pdf"
	default:
		return nil, "", "", errors.New("仅支持 JPG、PNG 或 PDF 格式的转账凭证")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, "", "", errors.New("无法读取转账凭证")
	}
	data, err := io.ReadAll(io.LimitReader(file, service.MaxCorporateProofBytes+1))
	if err != nil || int64(len(data)) > service.MaxCorporateProofBytes {
		return nil, "", "", errors.New("无法读取转账凭证")
	}
	return data, mimeType, extension, nil
}
