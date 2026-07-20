package controller

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var supportedXznPayStatuses = map[string]struct{}{
	"WAIT_BUYER_PAY": {},
	"TRADE_SUCCESS":  {},
	"TRADE_CLOSED":   {},
	"TRADE_REFUND":   {},
	"TRADE_FREEZE":   {},
	"TRADE_UNFREEZE": {},
}

type XznPayRequest struct {
	Amount         int64 `json:"amount"`
	PayMethodIndex *int  `json:"pay_method_index"`
}

func getXznPayClient() (*service.XznPayClient, error) {
	return service.NewXznPayClient(service.XznPayConfig{
		GatewayURL: setting.XznPayGatewayURL,
		PID:        setting.XznPayPID,
		SignType:   setting.XznPaySignType,
		MD5Key:     setting.XznPayMD5Key,
		PrivateKey: setting.XznPayPrivateKey,
		PublicKey:  setting.XznPayPublicKey,
	}, nil)
}

func getXznPayCallbackAddress() string {
	callbackAddress := strings.TrimSpace(setting.XznPayCallbackAddress)
	if callbackAddress == "" {
		callbackAddress = service.GetCallbackAddress()
	}
	return strings.TrimRight(callbackAddress, "/")
}

func RequestXznPay(c *gin.Context) {
	if !isXznPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "XznPay 支付未启用"})
		return
	}

	var req XznPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PayMethodIndex == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	methods := setting.GetXznPayMethods()
	methodIndex := *req.PayMethodIndex
	if methodIndex < 0 || methodIndex >= len(methods) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付方式"})
		return
	}
	method := methods[methodIndex]
	minimumTopUp := int64(setting.XznPayMinTopUp)
	if int64(method.MinTopUp) > minimumTopUp {
		minimumTopUp = int64(method.MinTopUp)
	}
	if req.Amount < minimumTopUp {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", minimumTopUp)})
		return
	}

	userID := c.GetInt("id")
	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	tradeNo := fmt.Sprintf("XZN-%d-%d-%s", userID, time.Now().UnixMilli(), common.GetRandomString(6))
	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = decimal.NewFromInt(req.Amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
		if amount < 1 {
			amount = 1
		}
	}
	topUp := &model.TopUp{
		UserId:          userID,
		Amount:          amount,
		Money:           decimal.NewFromFloat(payMoney).Round(2).InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodXznPay,
		ProviderPayType: strings.TrimSpace(method.PayTypeCode),
		ProviderStatus:  "WAIT_BUYER_PAY",
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("XznPay 创建本地充值订单失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	client, err := getXznPayClient()
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentMethodXznPay, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("XznPay 客户端初始化失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付配置错误"})
		return
	}
	callbackAddress := getXznPayCallbackAddress()
	returnAddress := strings.TrimRight(system_setting.ServerAddress, "/") + "/console/topup?show_history=true"
	params := map[string]string{
		"out_trade_no": tradeNo,
		"total_amount": decimal.NewFromFloat(topUp.Money).StringFixed(2),
		"subject":      fmt.Sprintf("TUC%d", req.Amount),
		"paytype_code": topUp.ProviderPayType,
		"notify_url":   callbackAddress + "/api/xzn-pay/webhook",
		"return_url":   returnAddress,
		"client_ip":    c.ClientIP(),
	}
	if strings.TrimSpace(method.ChannelID) != "" {
		params["channel_id"] = strings.TrimSpace(method.ChannelID)
	}
	response, err := client.CreateOrder(c.Request.Context(), params)
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentMethodXznPay, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("XznPay 远端下单失败 user_id=%d trade_no=%s paytype_code=%s error=%q", userID, tradeNo, topUp.ProviderPayType, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	if err := model.UpdateTopUpProviderTradeNo(tradeNo, response.Data.TradeNo, model.PaymentMethodXznPay); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("XznPay 保存平台订单号失败 user_id=%d trade_no=%s provider_trade_no=%s error=%q", userID, tradeNo, response.Data.TradeNo, err.Error()))
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("XznPay 充值订单创建成功 user_id=%d trade_no=%s provider_trade_no=%s amount=%d money=%.2f paytype_code=%s", userID, tradeNo, response.Data.TradeNo, req.Amount, topUp.Money, topUp.ProviderPayType))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"payment_url": response.Data.PayURL,
			"order_id":    tradeNo,
		},
	})
}

func XznPayWebhook(c *gin.Context) {
	if !isXznPayWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook 被拒绝 reason=webhook_disabled client_ip=%s", c.ClientIP()))
		writeXznPayWebhookResult(c, false)
		return
	}
	params, err := parseXznPayWebhookParams(c.Request)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("XznPay webhook 表单解析失败 client_ip=%s error=%q", c.ClientIP(), err.Error()))
		writeXznPayWebhookResult(c, false)
		return
	}
	client, err := getXznPayClient()
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook 客户端初始化失败 client_ip=%s error=%q", c.ClientIP(), err.Error()))
		writeXznPayWebhookResult(c, false)
		return
	}
	if !client.VerifyNotify(params) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf(
			"XznPay webhook 验签失败 client_ip=%s content_type=%q callback_sign_type=%q has_sign=%t param_count=%d",
			c.ClientIP(), c.GetHeader("Content-Type"), params["sign_type"], strings.TrimSpace(params["sign"]) != "", len(params),
		))
		writeXznPayWebhookResult(c, false)
		return
	}
	if strings.TrimSpace(params["pid"]) != strings.TrimSpace(setting.XznPayPID) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook PID 不匹配 client_ip=%s callback_pid=%s", c.ClientIP(), params["pid"]))
		writeXznPayWebhookResult(c, false)
		return
	}
	tradeStatus := strings.TrimSpace(params["trade_status"])
	if _, ok := supportedXznPayStatuses[tradeStatus]; !ok {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook 状态不支持 client_ip=%s trade_status=%s", c.ClientIP(), tradeStatus))
		writeXznPayWebhookResult(c, false)
		return
	}
	tradeNo := strings.TrimSpace(params["out_trade_no"])
	providerTradeNo := strings.TrimSpace(params["trade_no"])
	if providerTradeNo == "" {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook 缺少平台订单号 client_ip=%s trade_no=%s", c.ClientIP(), tradeNo))
		writeXznPayWebhookResult(c, false)
		return
	}
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentMethod != model.PaymentMethodXznPay {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook 本地订单不存在或支付方式不匹配 client_ip=%s trade_no=%s", c.ClientIP(), tradeNo))
		writeXznPayWebhookResult(c, false)
		return
	}
	if topUp.ProviderPayType != strings.TrimSpace(params["paytype_code"]) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook 支付类型不匹配 trade_no=%s expected=%s actual=%s", tradeNo, topUp.ProviderPayType, params["paytype_code"]))
		writeXznPayWebhookResult(c, false)
		return
	}
	totalAmountCents, err := parseXznPayMoneyCents(params["total_amount"])
	if err != nil || totalAmountCents != decimal.NewFromFloat(topUp.Money).Round(2).Shift(2).IntPart() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook 金额不匹配 trade_no=%s callback_amount=%s expected=%.2f error=%v", tradeNo, params["total_amount"], topUp.Money, err))
		writeXznPayWebhookResult(c, false)
		return
	}
	refundAmountCents := int64(0)
	if tradeStatus == "TRADE_REFUND" {
		refundAmountCents, err = parseXznPayMoneyCents(params["refund_amount"])
		if err != nil || refundAmountCents <= 0 {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("XznPay webhook 退款金额无效 trade_no=%s refund_amount=%s error=%v", tradeNo, params["refund_amount"], err))
			writeXznPayWebhookResult(c, false)
			return
		}
	}
	eventTime := getXznPayEventTime(params, tradeStatus)
	eventKey := buildXznPayEventKey(params, tradeStatus, eventTime)
	result, err := model.ApplyXznPayEvent(model.XznPayEventInput{
		EventKey:          eventKey,
		TradeNo:           tradeNo,
		ProviderTradeNo:   providerTradeNo,
		ProviderStatus:    tradeStatus,
		RefundAmountCents: refundAmountCents,
		EventTime:         eventTime,
		Payload:           common.GetJsonString(params),
	}, c.ClientIP())
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("XznPay webhook 账务处理失败 trade_no=%s trade_status=%s client_ip=%s error=%q", tradeNo, tradeStatus, c.ClientIP(), err.Error()))
		writeXznPayWebhookResult(c, false)
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("XznPay webhook 处理成功 trade_no=%s trade_status=%s duplicate=%t quota_delta=%d status=%s client_ip=%s", tradeNo, tradeStatus, result.Duplicate, result.QuotaDelta, result.Status, c.ClientIP()))
	writeXznPayWebhookResult(c, true)
}

func parseXznPayWebhookParams(request *http.Request) (map[string]string, error) {
	err := request.ParseMultipartForm(1 << 20)
	if err != nil && err != http.ErrNotMultipart {
		return nil, err
	}

	params := make(map[string]string, len(request.PostForm))
	for key := range request.PostForm {
		params[key] = request.PostForm.Get(key)
	}
	return params, nil
}

func parseXznPayMoneyCents(value string) (int64, error) {
	money, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil || money.IsNegative() || !money.Equal(money.Round(2)) {
		return 0, errorsNewXznPayAmount()
	}
	return money.Shift(2).IntPart(), nil
}

func errorsNewXznPayAmount() error {
	return fmt.Errorf("金额格式无效")
}

func getXznPayEventTime(params map[string]string, status string) int64 {
	keys := []string{"timestamp"}
	if status == "TRADE_SUCCESS" {
		keys = []string{"success_time", "timestamp"}
	} else if status == "TRADE_REFUND" {
		keys = []string{"refund_time", "timestamp"}
	}
	for _, key := range keys {
		if value, err := strconv.ParseInt(strings.TrimSpace(params[key]), 10, 64); err == nil && value > 0 {
			return value
		}
	}
	return common.GetTimestamp()
}

func buildXznPayEventKey(params map[string]string, status string, eventTime int64) string {
	eventIdentity := strconv.FormatInt(eventTime, 10)
	if status == "TRADE_SUCCESS" && strings.TrimSpace(params["success_time"]) != "" {
		eventIdentity = strings.TrimSpace(params["success_time"])
	}
	if status == "TRADE_REFUND" {
		eventIdentity = strings.TrimSpace(params["refund_time"])
	}
	parts := []string{
		strings.TrimSpace(params["trade_no"]),
		strings.TrimSpace(params["out_trade_no"]),
		status,
		eventIdentity,
		strings.TrimSpace(params["transaction_id"]),
		strings.TrimSpace(params["refund_amount"]),
	}
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", hash[:])
}

func writeXznPayWebhookResult(c *gin.Context, success bool) {
	result := "fail"
	if success {
		result = "success"
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(result))
}
