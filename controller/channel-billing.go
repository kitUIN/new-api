package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"

	"github.com/gin-gonic/gin"
)

// https://github.com/songquanpeng/one-api/issues/79

type OpenAISubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

type OpenAIUsageDailyCost struct {
	Timestamp float64 `json:"timestamp"`
	LineItems []struct {
		Name string  `json:"name"`
		Cost float64 `json:"cost"`
	}
}

type OpenAICreditGrants struct {
	Object         string  `json:"object"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalAvailable float64 `json:"total_available"`
}

type OpenAIUsageResponse struct {
	Object string `json:"object"`
	//DailyCosts []OpenAIUsageDailyCost `json:"daily_costs"`
	TotalUsage float64 `json:"total_usage"` // unit: 0.01 dollar
}

type OpenAISBUsageResponse struct {
	Msg  string `json:"msg"`
	Data *struct {
		Credit string `json:"credit"`
	} `json:"data"`
}

type AIProxyUserOverviewResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ErrorCode int    `json:"error_code"`
	Data      struct {
		TotalPoints float64 `json:"totalPoints"`
	} `json:"data"`
}

type API2GPTUsageResponse struct {
	Object         string  `json:"object"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalRemaining float64 `json:"total_remaining"`
}

type APGC2DGPTUsageResponse struct {
	//Grants         interface{} `json:"grants"`
	Object         string  `json:"object"`
	TotalAvailable float64 `json:"total_available"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
}

type SiliconFlowUsageResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  bool   `json:"status"`
	Data    struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Image         string `json:"image"`
		Email         string `json:"email"`
		IsAdmin       bool   `json:"isAdmin"`
		Balance       string `json:"balance"`
		Status        string `json:"status"`
		Introduction  string `json:"introduction"`
		Role          string `json:"role"`
		ChargeBalance string `json:"chargeBalance"`
		TotalBalance  string `json:"totalBalance"`
		Category      string `json:"category"`
	} `json:"data"`
}

type DeepSeekUsageResponse struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    string `json:"total_balance"`
		GrantedBalance  string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

type OpenRouterCreditResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

// GetAuthHeader get auth header
func GetAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	return h
}

// GetClaudeAuthHeader get claude auth header
func GetClaudeAuthHeader(token string) http.Header {
	h := http.Header{}
	h.Add("x-api-key", token)
	h.Add("anthropic-version", "2023-06-01")
	return h
}

func GetResponseBody(method, url string, channel *model.Channel, headers http.Header) ([]byte, error) {
	return GetResponseBodyWithBody(method, url, "", channel, headers)
}

func GetResponseBodyWithBody(method, url string, body string, channel *model.Channel, headers http.Header) ([]byte, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	for k := range headers {
		req.Header.Add(k, headers.Get(k))
	}
	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, &responseStatusError{StatusCode: res.StatusCode}
	}
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return responseBody, nil
}

func updateChannelCloseAIBalance(channel *model.Channel) (float64, error) {
	url := fmt.Sprintf("%s/dashboard/billing/credit_grants", channel.GetBaseURL())
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))

	if err != nil {
		return 0, err
	}
	response := OpenAICreditGrants{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalAvailable)
	return response.TotalAvailable, nil
}

func updateChannelOpenAISBBalance(channel *model.Channel) (float64, error) {
	url := fmt.Sprintf("https://api.openai-sb.com/sb-api/user/status?api_key=%s", channel.Key)
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := OpenAISBUsageResponse{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if response.Data == nil {
		return 0, errors.New(response.Msg)
	}
	balance, err := strconv.ParseFloat(response.Data.Credit, 64)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelAIProxyBalance(channel *model.Channel) (float64, error) {
	url := "https://aiproxy.io/api/report/getUserOverview"
	headers := http.Header{}
	headers.Add("Api-Key", channel.Key)
	body, err := GetResponseBody("GET", url, channel, headers)
	if err != nil {
		return 0, err
	}
	response := AIProxyUserOverviewResponse{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if !response.Success {
		return 0, fmt.Errorf("code: %d, message: %s", response.ErrorCode, response.Message)
	}
	channel.UpdateBalance(response.Data.TotalPoints)
	return response.Data.TotalPoints, nil
}

func updateChannelAPI2GPTBalance(channel *model.Channel) (float64, error) {
	url := "https://api.api2gpt.com/dashboard/billing/credit_grants"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))

	if err != nil {
		return 0, err
	}
	response := API2GPTUsageResponse{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalRemaining)
	return response.TotalRemaining, nil
}

func updateChannelSiliconFlowBalance(channel *model.Channel) (float64, error) {
	url := "https://api.siliconflow.cn/v1/user/info"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := SiliconFlowUsageResponse{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if response.Code != 20000 {
		return 0, fmt.Errorf("code: %d, message: %s", response.Code, response.Message)
	}
	balance, err := strconv.ParseFloat(response.Data.TotalBalance, 64)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelDeepSeekBalance(channel *model.Channel) (float64, error) {
	url := "https://api.deepseek.com/user/balance"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := DeepSeekUsageResponse{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	index := -1
	for i, balanceInfo := range response.BalanceInfos {
		if balanceInfo.Currency == "CNY" {
			index = i
			break
		}
	}
	if index == -1 {
		return 0, errors.New("currency CNY not found")
	}
	balance, err := strconv.ParseFloat(response.BalanceInfos[index].TotalBalance, 64)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelAIGC2DBalance(channel *model.Channel) (float64, error) {
	url := "https://api.aigc2d.com/dashboard/billing/credit_grants"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := APGC2DGPTUsageResponse{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	channel.UpdateBalance(response.TotalAvailable)
	return response.TotalAvailable, nil
}

func updateChannelOpenRouterBalance(channel *model.Channel) (float64, error) {
	url := "https://openrouter.ai/api/v1/credits"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	response := OpenRouterCreditResponse{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	balance := response.Data.TotalCredits - response.Data.TotalUsage
	channel.UpdateBalance(balance)
	return balance, nil
}

func updateChannelMoonshotBalance(channel *model.Channel) (float64, error) {
	url := "https://api.moonshot.cn/v1/users/me/balance"
	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}

	type MoonshotBalanceData struct {
		AvailableBalance float64 `json:"available_balance"`
		VoucherBalance   float64 `json:"voucher_balance"`
		CashBalance      float64 `json:"cash_balance"`
	}

	type MoonshotBalanceResponse struct {
		Code   int                 `json:"code"`
		Data   MoonshotBalanceData `json:"data"`
		Scode  string              `json:"scode"`
		Status bool                `json:"status"`
	}

	response := MoonshotBalanceResponse{}
	err = common.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}
	if !response.Status || response.Code != 0 {
		return 0, fmt.Errorf("failed to update moonshot balance, status: %v, code: %d, scode: %s", response.Status, response.Code, response.Scode)
	}
	availableBalanceCny := response.Data.AvailableBalance
	availableBalanceUsd := decimal.NewFromFloat(availableBalanceCny).Div(decimal.NewFromFloat(operation_setting.Price)).InexactFloat64()
	channel.UpdateBalance(availableBalanceUsd)
	return availableBalanceUsd, nil
}

const (
	balanceQueryTemplateNewAPI        = "newapi"
	balanceQueryTemplateSub2API       = "sub2api"
	balanceQueryDefaultIntervalSecond = 300
	channelLowBalanceNotifyThreshold  = 3.0
	channelLowBalanceNotifyCooldown   = 2 * time.Hour
	dailyProviderBalanceNotifyMaxRows = 30
)

var channelBalanceQueryTaskOnce sync.Once
var channelLowBalanceNotifyStore sync.Map
var providerLowBalanceNotifyStore sync.Map

type balanceQueryExecution struct {
	Channel  *model.Channel
	Settings dto.ChannelOtherSettings
}

type providerBalanceQueryExecution struct {
	Provider *model.ChannelProvider
	Channel  *model.Channel
	Settings dto.ChannelProviderSettings
}

type channelLowBalanceNotifyPayload struct {
	QQ      string `json:"qq"`
	AdminQQ string `json:"admin_qq"`
	To      string `json:"to"`
	Message string `json:"message"`
	Content string `json:"content"`
}

type balanceQueryDebugInfo struct {
	ChannelID int               `json:"channel_id"`
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body,omitempty"`
	Response  string            `json:"response,omitempty"`
	Error     string            `json:"error,omitempty"`
}

type responseStatusError struct {
	StatusCode int
}

func (e *responseStatusError) Error() string {
	return fmt.Sprintf("status code: %d", e.StatusCode)
}

type sub2APIAuthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	} `json:"data"`
}

type sub2APIAuthTokens struct {
	AccessToken  string
	RefreshToken string
}

func getBalanceQueryIntervalSeconds(config dto.BalanceQuery) int {
	if config.IntervalSeconds == nil {
		return balanceQueryDefaultIntervalSecond
	}
	if *config.IntervalSeconds < 0 {
		return balanceQueryDefaultIntervalSecond
	}
	return *config.IntervalSeconds
}

func normalizeBalanceQueryTemplate(template string) string {
	normalized := strings.ToLower(strings.TrimSpace(template))
	switch normalized {
	case "", "newapi", "new_api", "new-api":
		return balanceQueryTemplateNewAPI
	case "sub2api", "sub2_api", "sub2-api", "subapi", "sub_api", "sub-api":
		return balanceQueryTemplateSub2API
	default:
		return normalized
	}
}

func buildBalanceQueryConfig(channel *model.Channel, config dto.BalanceQuery) dto.BalanceQuery {
	template := normalizeBalanceQueryTemplate(config.Template)
	switch template {
	case balanceQueryTemplateNewAPI:
		config.Template = balanceQueryTemplateNewAPI
	case balanceQueryTemplateSub2API:
		config.Template = balanceQueryTemplateSub2API
	default:
		return config
	}

	if template == balanceQueryTemplateSub2API {
		if strings.TrimSpace(config.Request.URL) == "" {
			config.Request.URL = "{{baseUrl}}/api/v1/auth/me?timezone=Asia%2FShanghai"
		}
		if strings.TrimSpace(config.Request.Method) == "" {
			config.Request.Method = "GET"
		}
		if config.Request.Headers == nil {
			config.Request.Headers = map[string]string{}
		}
		if _, ok := config.Request.Headers["Authorization"]; !ok {
			config.Request.Headers["Authorization"] = "Bearer {{accessToken}}"
		}
		if strings.TrimSpace(config.Extractor.RemainingPath) == "" {
			config.Extractor.RemainingPath = "data.balance"
		}
		if strings.TrimSpace(config.Extractor.TotalPath) == "" {
			config.Extractor.TotalPath = "data.total_recharged"
		}
		if strings.TrimSpace(config.Extractor.UnitPath) == "" {
			config.Extractor.UnitPath = ""
		}
		if strings.TrimSpace(config.Extractor.Unit) == "" {
			config.Extractor.Unit = "USD"
		}
		if config.Extractor.Divisor == 0 {
			config.Extractor.Divisor = 1
		}
		if strings.TrimSpace(config.Extractor.SuccessPath) == "" {
			config.Extractor.SuccessPath = "code"
		}
		if strings.TrimSpace(config.Extractor.SuccessValue) == "" {
			config.Extractor.SuccessValue = "0"
		}
		config.Extractor.SuccessOptional = false
		_ = channel
		return config
	}

	if strings.TrimSpace(config.Request.URL) == "" {
		config.Request.URL = "{{baseUrl}}/api/user/self"
	}
	if strings.TrimSpace(config.Request.Method) == "" {
		config.Request.Method = "GET"
	}
	if config.Request.Headers == nil {
		config.Request.Headers = map[string]string{}
	}
	defaultHeaders := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer {{accessToken}}",
		"User-Agent":    "cc-switch/1.0",
		"New-Api-User":  "{{userId}}",
	}
	for key, value := range defaultHeaders {
		if _, ok := config.Request.Headers[key]; !ok {
			config.Request.Headers[key] = value
		}
	}
	if strings.TrimSpace(config.Extractor.PlanNamePath) == "" {
		config.Extractor.PlanNamePath = "data.group"
	}
	if strings.TrimSpace(config.Extractor.RemainingPath) == "" {
		config.Extractor.RemainingPath = "data.quota"
	}
	if strings.TrimSpace(config.Extractor.UsedPath) == "" {
		config.Extractor.UsedPath = "data.used_quota"
	}
	if strings.TrimSpace(config.Extractor.Unit) == "" {
		config.Extractor.Unit = "USD"
	}
	if config.Extractor.Divisor == 0 {
		config.Extractor.Divisor = 500000
	}
	if strings.TrimSpace(config.Extractor.SuccessPath) == "" {
		config.Extractor.SuccessPath = "success"
	}
	if strings.TrimSpace(config.Extractor.SuccessValue) == "" {
		config.Extractor.SuccessValue = "true"
	}
	if strings.TrimSpace(config.Extractor.MessagePath) == "" {
		config.Extractor.MessagePath = "message"
	}
	_ = channel
	return config
}

func replaceBalanceQueryVars(value string, channel *model.Channel, config dto.BalanceQuery) string {
	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(constant.ChannelBaseURLs[channel.Type], "/")
	}
	accessToken := config.AccessToken
	if accessToken == "" {
		accessToken = channel.Key
	}
	replacer := strings.NewReplacer(
		"{{baseUrl}}", baseURL,
		"{{accessToken}}", accessToken,
		"{{refreshToken}}", config.RefreshToken,
		"{{apiKey}}", channel.Key,
		"{{key}}", channel.Key,
		"{{userId}}", config.UserID,
		"{{channelId}}", strconv.Itoa(channel.Id),
		"{{channelName}}", channel.Name,
	)
	return replacer.Replace(value)
}

func isUnauthorizedStatus(err error) bool {
	var statusErr *responseStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusUnauthorized
}

func shouldRefreshSub2APIToken(template string, refreshToken string, err error) bool {
	return normalizeBalanceQueryTemplate(template) == balanceQueryTemplateSub2API &&
		strings.TrimSpace(refreshToken) != "" &&
		isUnauthorizedStatus(err)
}

func getQueryBaseURL(channel *model.Channel) string {
	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(constant.ChannelBaseURLs[channel.Type], "/")
	}
	return baseURL
}

func requestSub2APIAuthTokens(channel *model.Channel, path string, payload map[string]string, action string) (sub2APIAuthTokens, error) {
	baseURL := getQueryBaseURL(channel)
	if baseURL == "" {
		return sub2APIAuthTokens{}, fmt.Errorf("%s请求地址不能为空", action)
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return sub2APIAuthTokens{}, err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return sub2APIAuthTokens{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return sub2APIAuthTokens{}, err
	}
	res, err := client.Do(req)
	if err != nil {
		return sub2APIAuthTokens{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return sub2APIAuthTokens{}, &responseStatusError{StatusCode: res.StatusCode}
	}
	body, err = io.ReadAll(res.Body)
	if err != nil {
		return sub2APIAuthTokens{}, err
	}
	response := sub2APIAuthResponse{}
	if err := common.Unmarshal(body, &response); err != nil {
		return sub2APIAuthTokens{}, err
	}
	if response.Code != 0 {
		if strings.TrimSpace(response.Message) != "" {
			return sub2APIAuthTokens{}, errors.New(response.Message)
		}
		return sub2APIAuthTokens{}, fmt.Errorf("%s失败，code: %d", action, response.Code)
	}
	tokens := sub2APIAuthTokens{
		AccessToken:  strings.TrimSpace(response.Data.AccessToken),
		RefreshToken: strings.TrimSpace(response.Data.RefreshToken),
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		return sub2APIAuthTokens{}, fmt.Errorf("%s响应缺少 access_token 或 refresh_token", action)
	}
	return tokens, nil
}

func refreshSub2APIQueryToken(channel *model.Channel, refreshToken string) (sub2APIAuthTokens, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return sub2APIAuthTokens{}, errors.New("refresh_token 不能为空")
	}
	return requestSub2APIAuthTokens(channel, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	}, "刷新 token")
}

func loginSub2APIQueryToken(channel *model.Channel, email string, password string) (sub2APIAuthTokens, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return sub2APIAuthTokens{}, errors.New("sub2api 登录邮箱不能为空")
	}
	if password == "" {
		return sub2APIAuthTokens{}, errors.New("sub2api 登录密码不能为空")
	}
	return requestSub2APIAuthTokens(channel, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "sub2api 登录")
}

func updateChannelBalanceQueryTokens(channel *model.Channel, settings dto.ChannelOtherSettings, tokens sub2APIAuthTokens) dto.ChannelOtherSettings {
	settings.BalanceQuery.AccessToken = tokens.AccessToken
	settings.BalanceQuery.RefreshToken = tokens.RefreshToken
	settings.GroupQuery.AccessToken = tokens.AccessToken
	settings.GroupQuery.RefreshToken = tokens.RefreshToken
	channel.SetOtherSettings(settings)
	return settings
}

func updateProviderQueryTokens(provider *model.ChannelProvider, settings dto.ChannelProviderSettings, tokens sub2APIAuthTokens) dto.ChannelProviderSettings {
	settings.BalanceQuery.AccessToken = tokens.AccessToken
	settings.BalanceQuery.RefreshToken = tokens.RefreshToken
	settings.GroupQuery.AccessToken = tokens.AccessToken
	settings.GroupQuery.RefreshToken = tokens.RefreshToken
	provider.SetOtherSettings(settings)
	return settings
}

func updateChannelGroupQueryTokens(channel *model.Channel, settings dto.ChannelOtherSettings, tokens sub2APIAuthTokens) dto.ChannelOtherSettings {
	settings.BalanceQuery.AccessToken = tokens.AccessToken
	settings.BalanceQuery.RefreshToken = tokens.RefreshToken
	settings.GroupQuery.AccessToken = tokens.AccessToken
	settings.GroupQuery.RefreshToken = tokens.RefreshToken
	channel.SetOtherSettings(settings)
	return settings
}

func recoverSub2APIProviderQuery(
	channel *model.Channel,
	provider *model.ChannelProvider,
	settings dto.ChannelProviderSettings,
	template string,
	refreshToken string,
	queryErr error,
	retry func(tokens sub2APIAuthTokens) ([]byte, error),
) ([]byte, dto.ChannelProviderSettings, error) {
	if !shouldRefreshSub2APIToken(template, refreshToken, queryErr) {
		return nil, settings, queryErr
	}

	tokens, refreshErr := refreshSub2APIQueryToken(channel, refreshToken)
	if refreshErr == nil {
		settings = updateProviderQueryTokens(provider, settings, tokens)
		body, err := retry(tokens)
		if !isUnauthorizedStatus(err) {
			return body, settings, err
		}
		if !settings.Sub2APIAutoLoginEnabled {
			return body, settings, err
		}
	} else if !settings.Sub2APIAutoLoginEnabled || !isUnauthorizedStatus(refreshErr) {
		return nil, settings, fmt.Errorf("401 后刷新 token 失败: %w", refreshErr)
	}

	tokens, loginErr := loginSub2APIQueryToken(channel, settings.Sub2APIEmail, settings.Sub2APIPassword)
	if loginErr != nil {
		return nil, settings, fmt.Errorf("刷新 token 后仍为 401，sub2api 登录失败: %w", loginErr)
	}
	settings = updateProviderQueryTokens(provider, settings, tokens)
	body, err := retry(tokens)
	if isUnauthorizedStatus(err) {
		return nil, settings, fmt.Errorf("sub2api 登录后查询仍返回 401: %w", err)
	}
	return body, settings, err
}

func persistChannelSettingsOnly(channel *model.Channel, settings dto.ChannelOtherSettings) {
	channel.SetOtherSettings(settings)
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(map[string]interface{}{
		"settings": channel.OtherSettings,
	}).Error; err != nil {
		common.SysLog(fmt.Sprintf("failed to persist channel settings: channel_id=%d, error=%v", channel.Id, err))
	}
}

func balanceQueryHeadersToMap(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key := range headers {
		result[key] = headers.Get(key)
	}
	return result
}

func logBalanceQueryDebug(info balanceQueryDebugInfo) {
	data, err := common.Marshal(info)
	if err != nil {
		common.SysLog(fmt.Sprintf("balance query debug: channel_id=%d, marshal error=%v", info.ChannelID, err))
		return
	}
	common.SysLog("balance query debug: " + string(data))
}

func splitBalanceQueryPaths(path string) []string {
	parts := strings.Split(path, ",")
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}
	return paths
}

func getBalanceQueryJSONValue(body []byte, path string) (gjson.Result, bool) {
	for _, candidate := range splitBalanceQueryPaths(path) {
		result := gjson.GetBytes(body, candidate)
		if result.Exists() && result.Type != gjson.Null {
			return result, true
		}
	}
	return gjson.Result{}, false
}

func shouldSendChannelLowBalanceNotify(channelId int, now time.Time) bool {
	key := "channel_low_balance_notify"
	if common.RedisEnabled {
		if _, err := common.RedisGet(key); err == nil {
			return false
		}
		if err := common.RedisSet(key, "1", channelLowBalanceNotifyCooldown); err == nil {
			return true
		} else {
			common.SysLog(fmt.Sprintf("failed to set channel low balance notify redis key: channel_id=%d, error=%v", channelId, err))
		}
	}

	nowUnix := now.Unix()
	if value, ok := channelLowBalanceNotifyStore.Load(key); ok {
		if lastSentAt, ok := value.(int64); ok && nowUnix-lastSentAt < int64(channelLowBalanceNotifyCooldown.Seconds()) {
			return false
		}
	}
	channelLowBalanceNotifyStore.Store(key, nowUnix)
	return true
}

func shouldSendProviderLowBalanceNotify(providerId int, now time.Time) bool {
	key := fmt.Sprintf("provider_low_balance_notify:%d", providerId)
	if common.RedisEnabled {
		if _, err := common.RedisGet(key); err == nil {
			return false
		}
		if err := common.RedisSet(key, "1", channelLowBalanceNotifyCooldown); err == nil {
			return true
		} else {
			common.SysLog(fmt.Sprintf("failed to set provider low balance notify redis key: provider_id=%d, error=%v", providerId, err))
		}
	}

	nowUnix := now.Unix()
	if value, ok := providerLowBalanceNotifyStore.Load(key); ok {
		if lastSentAt, ok := value.(int64); ok && nowUnix-lastSentAt < int64(channelLowBalanceNotifyCooldown.Seconds()) {
			return false
		}
	}
	providerLowBalanceNotifyStore.Store(key, nowUnix)
	return true
}

func channelQQServiceURL(path string) string {
	return strings.TrimRight(common.QQCallbackAddress, "/") + path
}

func setChannelQQServiceAuthHeader(req *http.Request) {
	if common.QQCallbackAccessToken != "" {
		req.Header.Set("Authorization", common.QQCallbackAccessToken)
		req.Header.Set("X-Access-Token", common.QQCallbackAccessToken)
	}
}

func sendChannelLowBalanceNotify(channel *model.Channel, balance float64) {
	if channel == nil || balance >= channelLowBalanceNotifyThreshold {
		return
	}
	if common.QQCallbackAddress == "" || common.QQCallbackAccessToken == "" || common.QQAdminNumber == "" {
		return
	}
	if !shouldSendChannelLowBalanceNotify(channel.Id, time.Now()) {
		return
	}

	go func() {
		message := fmt.Sprintf("渠道余额低于 %.2f USD：渠道ID %d，名称 %s，当前余额 %.4f USD", channelLowBalanceNotifyThreshold, channel.Id, channel.Name, balance)
		payload, err := common.Marshal(channelLowBalanceNotifyPayload{
			QQ:      common.QQAdminNumber,
			AdminQQ: common.QQAdminNumber,
			To:      common.QQAdminNumber,
			Message: message,
			Content: message,
		})
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to marshal channel low balance notify payload: channel_id=%d, error=%v", channel.Id, err))
			return
		}
		req, err := http.NewRequest(http.MethodPost, channelQQServiceURL("/api/nachoai/send_message"), bytes.NewReader(payload))
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to create channel low balance notify request: channel_id=%d, error=%v", channel.Id, err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		setChannelQQServiceAuthHeader(req)
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to send channel low balance notify: channel_id=%d, error=%v", channel.Id, err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			common.SysLog(fmt.Sprintf("failed to send channel low balance notify: channel_id=%d, status=%d", channel.Id, resp.StatusCode))
		}
	}()
}

func sendQQAdminMessage(message string, logPrefix string) {
	if strings.TrimSpace(message) == "" || common.QQCallbackAddress == "" || common.QQCallbackAccessToken == "" || common.QQAdminNumber == "" {
		return
	}
	go func() {
		payload, err := common.Marshal(channelLowBalanceNotifyPayload{
			QQ:      common.QQAdminNumber,
			AdminQQ: common.QQAdminNumber,
			To:      common.QQAdminNumber,
			Message: message,
			Content: message,
		})
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to marshal %s payload: %v", logPrefix, err))
			return
		}
		req, err := http.NewRequest(http.MethodPost, channelQQServiceURL("/api/nachoai/send_message"), bytes.NewReader(payload))
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to create %s request: %v", logPrefix, err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		setChannelQQServiceAuthHeader(req)
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to send %s: %v", logPrefix, err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			common.SysLog(fmt.Sprintf("failed to send %s: status=%d", logPrefix, resp.StatusCode))
		}
	}()
}

func validateBalanceQuerySuccess(body []byte, extractor dto.BalanceQueryExtractorConfig) (bool, string) {
	if extractor.SuccessPath == "" {
		return true, ""
	}
	result, ok := getBalanceQueryJSONValue(body, extractor.SuccessPath)
	if !ok {
		if extractor.SuccessOptional {
			return true, ""
		}
		return false, "余额查询响应缺少成功状态字段"
	}
	expected := strings.TrimSpace(extractor.SuccessValue)
	if expected == "" {
		if result.Type == gjson.True {
			return true, ""
		}
		return false, "余额查询响应状态为失败"
	}
	actual := strings.TrimSpace(result.String())
	if result.Type == gjson.True {
		actual = "true"
	} else if result.Type == gjson.False {
		actual = "false"
	}
	if actual == expected {
		return true, ""
	}
	return false, "余额查询响应状态为失败"
}

func getBalanceQueryNumber(body []byte, path string, divisor float64) (float64, bool) {
	if path == "" {
		return 0, false
	}
	result, ok := getBalanceQueryJSONValue(body, path)
	if !ok {
		return 0, false
	}
	value := result.Float()
	if divisor != 0 {
		value = value / divisor
	}
	return value, true
}

func extractBalanceQueryResult(body []byte, extractor dto.BalanceQueryExtractorConfig) dto.BalanceQueryResult {
	divisor := extractor.Divisor
	if divisor == 0 {
		divisor = 1
	}
	result := dto.BalanceQueryResult{
		IsValid:   true,
		Unit:      extractor.Unit,
		CheckedAt: common.GetTimestamp(),
	}
	if result.Unit == "" {
		result.Unit = "USD"
	}
	if ok, message := validateBalanceQuerySuccess(body, extractor); !ok {
		if extractor.MessagePath != "" {
			if upstreamMessage := strings.TrimSpace(gjson.GetBytes(body, extractor.MessagePath).String()); upstreamMessage != "" {
				message = upstreamMessage
			}
		}
		result.IsValid = false
		result.InvalidMessage = message
		return result
	}
	if extractor.PlanNamePath != "" {
		if value, ok := getBalanceQueryJSONValue(body, extractor.PlanNamePath); ok {
			result.PlanName = strings.TrimSpace(value.String())
		}
	}
	if result.PlanName == "" {
		result.PlanName = "默认套餐"
	}
	if extractor.UnitPath != "" {
		if unit, ok := getBalanceQueryJSONValue(body, extractor.UnitPath); ok {
			if unitValue := strings.TrimSpace(unit.String()); unitValue != "" {
				result.Unit = unitValue
			}
		}
	}
	if remaining, ok := getBalanceQueryNumber(body, extractor.RemainingPath, divisor); ok {
		result.Remaining = remaining
	} else {
		result.IsValid = false
		result.InvalidMessage = "余额查询响应缺少剩余额度字段"
		return result
	}
	hasUsed := false
	if used, ok := getBalanceQueryNumber(body, extractor.UsedPath, divisor); ok {
		result.Used = used
		hasUsed = true
	}
	if total, ok := getBalanceQueryNumber(body, extractor.TotalPath, divisor); ok {
		result.Total = total
		if !hasUsed {
			result.Used = result.Total - result.Remaining
			if result.Used < 0 {
				result.Used = 0
			}
		}
	} else {
		result.Total = result.Remaining + result.Used
	}
	return result
}

func persistBalanceQueryResult(channel *model.Channel, settings dto.ChannelOtherSettings, result dto.BalanceQueryResult, err error) {
	settings.BalanceQuery.LastCheckTime = common.GetTimestamp()
	settings.BalanceQuery.LastResult = &result
	if err != nil {
		settings.BalanceQuery.LastError = err.Error()
	} else if !result.IsValid {
		settings.BalanceQuery.LastError = result.InvalidMessage
	} else {
		settings.BalanceQuery.LastError = ""
	}
	channel.SetOtherSettings(settings)
	updates := map[string]interface{}{
		"settings": channel.OtherSettings,
	}
	if err == nil && result.IsValid {
		channel.Balance = result.Remaining
		channel.BalanceUpdatedTime = common.GetTimestamp()
		updates["balance"] = channel.Balance
		updates["balance_updated_time"] = channel.BalanceUpdatedTime
	}
	if updateErr := model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(updates).Error; updateErr != nil {
		common.SysLog(fmt.Sprintf("failed to persist balance query result: channel_id=%d, error=%v", channel.Id, updateErr))
	}
}

func persistProviderBalanceQueryResult(provider *model.ChannelProvider, settings dto.ChannelProviderSettings, result dto.BalanceQueryResult, err error) {
	settings.BalanceQuery.LastCheckTime = common.GetTimestamp()
	settings.BalanceQuery.LastResult = &result
	if err != nil {
		settings.BalanceQuery.LastError = err.Error()
	} else if !result.IsValid {
		settings.BalanceQuery.LastError = result.InvalidMessage
	} else {
		settings.BalanceQuery.LastError = ""
	}
	provider.SetOtherSettings(settings)
	updates := map[string]interface{}{
		"settings":     provider.Settings,
		"updated_time": common.GetTimestamp(),
	}
	if err == nil && result.IsValid {
		provider.Balance = result.Remaining
		provider.BalanceUpdatedTime = common.GetTimestamp()
		updates["balance"] = provider.Balance
		updates["balance_updated_time"] = provider.BalanceUpdatedTime
	}
	if updateErr := model.DB.Model(&model.ChannelProvider{}).Where("id = ?", provider.Id).Updates(updates).Error; updateErr != nil {
		common.SysLog(fmt.Sprintf("failed to persist provider balance query result: provider_id=%d, error=%v", provider.Id, updateErr))
	}
}

func updateChannelConfiguredBalance(channel *model.Channel) (float64, *dto.BalanceQueryResult, bool, error) {
	settings := channel.GetOtherSettings()
	config := settings.BalanceQuery
	if !config.Enabled {
		return 0, nil, false, nil
	}
	execution := balanceQueryExecution{Channel: channel, Settings: settings}
	if config.SourceChannelID > 0 && config.SourceChannelID != channel.Id {
		sourceChannel, err := model.GetChannelById(config.SourceChannelID, true)
		if err != nil {
			result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: err.Error(), CheckedAt: common.GetTimestamp()}
			persistBalanceQueryResult(channel, settings, result, err)
			return 0, &result, true, err
		}
		sourceSettings := sourceChannel.GetOtherSettings()
		if !sourceSettings.BalanceQuery.Enabled || sourceSettings.BalanceQuery.SourceChannelID > 0 {
			err := errors.New("共享余额查询源渠道未启用独立余额查询配置")
			result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: err.Error(), CheckedAt: common.GetTimestamp()}
			persistBalanceQueryResult(channel, settings, result, err)
			return 0, &result, true, err
		}
		execution = balanceQueryExecution{Channel: sourceChannel, Settings: sourceSettings}
	}
	return executeChannelConfiguredBalance(channel, settings, execution)
}

func getProviderQuerySourceChannel(provider *model.ChannelProvider, sourceChannelID int) (*model.Channel, error) {
	if provider == nil {
		return nil, errors.New("供应商不存在")
	}
	if sourceChannelID > 0 {
		channel, err := model.GetChannelById(sourceChannelID, true)
		if err != nil {
			return nil, err
		}
		if channel.ProviderID != provider.Id {
			return nil, errors.New("查询源渠道不属于该供应商")
		}
		if channel.ChannelInfo.IsMultiKey {
			return nil, errors.New("多密钥渠道不支持作为供应商查询源")
		}
		return channel, nil
	}
	var channels []*model.Channel
	if err := model.DB.Where("provider_id = ? AND status = ?", provider.Id, common.ChannelStatusEnabled).Order("id asc").Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, channel := range channels {
		if !channel.ChannelInfo.IsMultiKey {
			return channel, nil
		}
	}
	if err := model.DB.Where("provider_id = ?", provider.Id).Order("id asc").Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, channel := range channels {
		if !channel.ChannelInfo.IsMultiKey {
			return channel, nil
		}
	}
	return nil, errors.New("供应商下没有可用的单密钥查询源渠道")
}

func updateProviderConfiguredBalance(provider *model.ChannelProvider) (float64, *dto.BalanceQueryResult, bool, error) {
	settings := provider.GetOtherSettings()
	config := settings.BalanceQuery
	if !config.Enabled {
		return 0, nil, false, nil
	}
	sourceChannel, err := getProviderQuerySourceChannel(provider, config.SourceChannelID)
	if err != nil {
		result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: err.Error(), CheckedAt: common.GetTimestamp()}
		persistProviderBalanceQueryResult(provider, settings, result, err)
		return 0, &result, true, err
	}
	return executeProviderConfiguredBalance(provider, settings, providerBalanceQueryExecution{Provider: provider, Channel: sourceChannel, Settings: settings})
}

func executeProviderConfiguredBalance(provider *model.ChannelProvider, providerSettings dto.ChannelProviderSettings, execution providerBalanceQueryExecution) (float64, *dto.BalanceQueryResult, bool, error) {
	channel := execution.Channel
	config := buildBalanceQueryConfig(channel, execution.Settings.BalanceQuery)
	method := strings.ToUpper(strings.TrimSpace(config.Request.Method))
	if method == "" {
		method = http.MethodGet
	}
	url := replaceBalanceQueryVars(config.Request.URL, channel, config)
	if strings.TrimSpace(url) == "" {
		err := errors.New("余额查询请求地址不能为空")
		result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: err.Error(), CheckedAt: common.GetTimestamp()}
		persistProviderBalanceQueryResult(provider, providerSettings, result, err)
		return 0, &result, true, err
	}
	headers := http.Header{}
	for key, value := range config.Request.Headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		headers.Set(key, replaceBalanceQueryVars(value, channel, config))
	}
	requestBody := replaceBalanceQueryVars(config.Request.Body, channel, config)
	debugInfo := balanceQueryDebugInfo{
		ChannelID: channel.Id,
		URL:       url,
		Method:    method,
		Headers:   balanceQueryHeadersToMap(headers),
		Body:      requestBody,
	}
	body, err := GetResponseBodyWithBody(method, url, requestBody, channel, headers)
	if err != nil {
		retryWithTokens := func(tokens sub2APIAuthTokens) ([]byte, error) {
			config.AccessToken = tokens.AccessToken
			config.RefreshToken = tokens.RefreshToken
			headers = http.Header{}
			for key, value := range config.Request.Headers {
				if strings.TrimSpace(key) == "" {
					continue
				}
				headers.Set(key, replaceBalanceQueryVars(value, channel, config))
			}
			requestBody = replaceBalanceQueryVars(config.Request.Body, channel, config)
			debugInfo.Headers = balanceQueryHeadersToMap(headers)
			debugInfo.Body = requestBody
			return GetResponseBodyWithBody(method, url, requestBody, channel, headers)
		}
		body, providerSettings, err = recoverSub2APIProviderQuery(
			channel,
			provider,
			providerSettings,
			config.Template,
			config.RefreshToken,
			err,
			retryWithTokens,
		)
	}
	if err != nil {
		debugInfo.Error = err.Error()
		logBalanceQueryDebug(debugInfo)
		result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: err.Error(), CheckedAt: common.GetTimestamp()}
		persistProviderBalanceQueryResult(provider, providerSettings, result, err)
		return 0, &result, true, err
	}
	debugInfo.Response = string(body)
	result := extractBalanceQueryResult(body, config.Extractor)
	if !result.IsValid {
		err = errors.New(result.InvalidMessage)
		debugInfo.Error = err.Error()
		logBalanceQueryDebug(debugInfo)
		persistProviderBalanceQueryResult(provider, providerSettings, result, err)
		return 0, &result, true, err
	}
	persistProviderBalanceQueryResult(provider, providerSettings, result, nil)
	return result.Remaining, &result, true, nil
}

func executeChannelConfiguredBalance(target *model.Channel, targetSettings dto.ChannelOtherSettings, execution balanceQueryExecution) (float64, *dto.BalanceQueryResult, bool, error) {
	channel := execution.Channel
	settings := execution.Settings
	config := settings.BalanceQuery
	config = buildBalanceQueryConfig(channel, config)
	method := strings.ToUpper(strings.TrimSpace(config.Request.Method))
	if method == "" {
		method = http.MethodGet
	}
	url := replaceBalanceQueryVars(config.Request.URL, channel, config)
	if strings.TrimSpace(url) == "" {
		err := errors.New("余额查询请求地址不能为空")
		result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: err.Error(), CheckedAt: common.GetTimestamp()}
		persistBalanceQueryResult(target, targetSettings, result, err)
		return 0, &result, true, err
	}
	headers := http.Header{}
	for key, value := range config.Request.Headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		headers.Set(key, replaceBalanceQueryVars(value, channel, config))
	}
	requestBody := replaceBalanceQueryVars(config.Request.Body, channel, config)
	debugInfo := balanceQueryDebugInfo{
		ChannelID: target.Id,
		URL:       url,
		Method:    method,
		Headers:   balanceQueryHeadersToMap(headers),
		Body:      requestBody,
	}
	body, err := GetResponseBodyWithBody(method, url, requestBody, channel, headers)
	if err != nil {
		if shouldRefreshSub2APIToken(config.Template, config.RefreshToken, err) {
			tokens, refreshErr := refreshSub2APIQueryToken(channel, config.RefreshToken)
			if refreshErr == nil {
				settings = updateChannelBalanceQueryTokens(channel, settings, tokens)
				if target.Id == channel.Id {
					targetSettings = settings
				} else {
					persistChannelSettingsOnly(channel, settings)
				}
				config.AccessToken = tokens.AccessToken
				config.RefreshToken = tokens.RefreshToken
				headers = http.Header{}
				for key, value := range config.Request.Headers {
					if strings.TrimSpace(key) == "" {
						continue
					}
					headers.Set(key, replaceBalanceQueryVars(value, channel, config))
				}
				requestBody = replaceBalanceQueryVars(config.Request.Body, channel, config)
				debugInfo.Headers = balanceQueryHeadersToMap(headers)
				debugInfo.Body = requestBody
				body, err = GetResponseBodyWithBody(method, url, requestBody, channel, headers)
			} else {
				err = fmt.Errorf("余额查询 401 后刷新 token 失败: %w", refreshErr)
			}
		}
	}
	if err != nil {
		debugInfo.Error = err.Error()
		logBalanceQueryDebug(debugInfo)
		result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: err.Error(), CheckedAt: common.GetTimestamp()}
		persistBalanceQueryResult(target, targetSettings, result, err)
		return 0, &result, true, err
	}
	debugInfo.Response = string(body)
	result := extractBalanceQueryResult(body, config.Extractor)
	if !result.IsValid {
		err = errors.New(result.InvalidMessage)
		debugInfo.Error = err.Error()
		logBalanceQueryDebug(debugInfo)
		persistBalanceQueryResult(target, targetSettings, result, err)
		return 0, &result, true, err
	}
	persistBalanceQueryResult(target, targetSettings, result, nil)
	return result.Remaining, &result, true, nil
}

func updateChannelBalance(channel *model.Channel) (float64, error) {
	if balance, _, configured, err := updateChannelConfiguredBalance(channel); configured {
		if err == nil {
			sendChannelLowBalanceNotify(channel, balance)
		}
		return balance, err
	}
	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() == "" {
		channel.BaseURL = &baseURL
	}
	switch channel.Type {
	case constant.ChannelTypeOpenAI:
		if channel.GetBaseURL() != "" {
			baseURL = channel.GetBaseURL()
		}
	case constant.ChannelTypeAzure:
		return 0, errors.New("尚未实现")
	case constant.ChannelTypeCustom:
		baseURL = channel.GetBaseURL()
	//case common.ChannelTypeOpenAISB:
	//	return updateChannelOpenAISBBalance(channel)
	case constant.ChannelTypeAIProxy:
		return updateChannelAIProxyBalance(channel)
	case constant.ChannelTypeAPI2GPT:
		return updateChannelAPI2GPTBalance(channel)
	case constant.ChannelTypeAIGC2D:
		return updateChannelAIGC2DBalance(channel)
	case constant.ChannelTypeSiliconFlow:
		return updateChannelSiliconFlowBalance(channel)
	case constant.ChannelTypeDeepSeek:
		return updateChannelDeepSeekBalance(channel)
	case constant.ChannelTypeOpenRouter:
		return updateChannelOpenRouterBalance(channel)
	case constant.ChannelTypeMoonshot:
		return updateChannelMoonshotBalance(channel)
	default:
		return 0, errors.New("尚未实现")
	}
	url := fmt.Sprintf("%s/v1/dashboard/billing/subscription", baseURL)

	body, err := GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	subscription := OpenAISubscriptionResponse{}
	err = common.Unmarshal(body, &subscription)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}
	url = fmt.Sprintf("%s/v1/dashboard/billing/usage?start_date=%s&end_date=%s", baseURL, startDate, endDate)
	body, err = GetResponseBody("GET", url, channel, GetAuthHeader(channel.Key))
	if err != nil {
		return 0, err
	}
	usage := OpenAIUsageResponse{}
	err = common.Unmarshal(body, &usage)
	if err != nil {
		return 0, err
	}
	balance := subscription.HardLimitUSD - usage.TotalUsage/100
	channel.UpdateBalance(balance)
	sendChannelLowBalanceNotify(channel, balance)
	return balance, nil
}

func updateProviderBalance(provider *model.ChannelProvider) (float64, error) {
	if balance, _, configured, err := updateProviderConfiguredBalance(provider); configured {
		return balance, err
	}
	return 0, errors.New("供应商未启用余额查询配置")
}

func UpdateChannelBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := model.CacheGetChannel(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if channel.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "多密钥渠道不支持余额查询",
		})
		return
	}
	balance, err := updateChannelBalance(channel)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"balance": balance,
		"data": gin.H{
			"settings":             channel.OtherSettings,
			"balance_updated_time": channel.BalanceUpdatedTime,
		},
	})
}

func UpdateChannelProviderBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	provider, err := model.GetChannelProviderByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	balance, err := updateProviderBalance(provider)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"balance": balance,
		"data": gin.H{
			"settings":             provider.Settings,
			"balance_updated_time": provider.BalanceUpdatedTime,
		},
	})
}

func GetChannelBalanceQueryInstances(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    []gin.H{},
	})
}

func shouldRunProviderBalanceQuery(provider *model.ChannelProvider, now int64) bool {
	settings := provider.GetOtherSettings()
	config := settings.BalanceQuery
	if !config.Enabled {
		return false
	}
	interval := getBalanceQueryIntervalSeconds(config)
	if interval == 0 {
		return false
	}
	return config.LastCheckTime <= 0 || now-config.LastCheckTime >= int64(interval)
}

func shouldRunChannelBalanceQuery(channel *model.Channel, now int64) bool {
	settings := channel.GetOtherSettings()
	config := settings.BalanceQuery
	if !config.Enabled {
		return false
	}
	interval := getBalanceQueryIntervalSeconds(config)
	if interval == 0 {
		return false
	}
	return config.LastCheckTime <= 0 || now-config.LastCheckTime >= int64(interval)
}

func updateConfiguredChannelsBalanceDue() error {
	providers, _, err := model.ListChannelProviders(0, 0)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	for _, provider := range providers {
		if provider.Status != common.ChannelStatusEnabled {
			continue
		}
		if !shouldRunProviderBalanceQuery(provider, now) {
			continue
		}
		settings := provider.GetOtherSettings()
		oldResult := cloneBalanceQueryResult(settings.BalanceQuery.LastResult)
		balance, err := updateProviderBalance(provider)
		updated, getErr := model.GetChannelProviderByID(provider.Id)
		if getErr != nil {
			common.SysLog(fmt.Sprintf("failed to reload provider after scheduled balance query: provider_id=%d, error=%v", provider.Id, getErr))
			time.Sleep(common.RequestInterval)
			continue
		}
		newSettings := updated.GetOtherSettings()
		newResult := cloneBalanceQueryResult(newSettings.BalanceQuery.LastResult)
		if err == nil && isProviderBalanceLowChanged(oldResult, newResult) {
			sendProviderLowBalanceChangedNotify(providerBalanceLowBalanceChange{
				ProviderID: provider.Id,
				Name:       provider.Name,
				OldBalance: oldResult.Remaining,
				NewBalance: newResult.Remaining,
				PlanName:   balanceQueryDisplayPlanName(newResult),
				Unit:       balanceQueryDisplayUnit(newResult),
			})
		}
		if err == nil && balance <= 0 {
			disableProviderChannelsForBalance(provider.Id)
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

type providerBalanceLowBalanceChange struct {
	ProviderID int
	Name       string
	OldBalance float64
	NewBalance float64
	PlanName   string
	Unit       string
}

type providerBalanceDailyReport struct {
	ProviderID   int
	Name         string
	Result       *dto.BalanceQueryResult
	DailyUsed    float64
	HasDailyUsed bool
	LastError    string
	QueryError   error
}

func cloneBalanceQueryResult(result *dto.BalanceQueryResult) *dto.BalanceQueryResult {
	if result == nil {
		return nil
	}
	copied := *result
	return &copied
}

func calculateBalanceQueryDailyUsed(previousDailyResult, currentResult *dto.BalanceQueryResult) (float64, bool) {
	if currentResult == nil || !currentResult.IsValid {
		return 0, false
	}
	if previousDailyResult == nil || !previousDailyResult.IsValid {
		return 0, false
	}
	dailyUsed := currentResult.Used - previousDailyResult.Used
	if dailyUsed < 0 {
		dailyUsed = currentResult.Used
	}
	if dailyUsed < 0 {
		dailyUsed = 0
	}
	return dailyUsed, true
}

func balanceQueryFloatChanged(oldValue, newValue float64) bool {
	return math.Abs(oldValue-newValue) >= 0.000001
}

func isProviderBalanceLowChanged(oldResult, newResult *dto.BalanceQueryResult) bool {
	return oldResult != nil &&
		newResult != nil &&
		newResult.IsValid &&
		newResult.Remaining < channelLowBalanceNotifyThreshold &&
		balanceQueryFloatChanged(oldResult.Remaining, newResult.Remaining)
}

func sendProviderLowBalanceChangedNotify(change providerBalanceLowBalanceChange) {
	if !shouldSendProviderLowBalanceNotify(change.ProviderID, time.Now()) {
		return
	}
	message := fmt.Sprintf(
		"供应商余额低于 %.2f USD 且金额发生变化：供应商ID %d，%s（%s），余额 %.4f -> %.4f %s",
		channelLowBalanceNotifyThreshold,
		change.ProviderID,
		change.Name,
		change.PlanName,
		change.OldBalance,
		change.NewBalance,
		change.Unit,
	)
	sendQQAdminMessage(message, "provider low balance notify")
}

func balanceQueryDisplayUnit(result *dto.BalanceQueryResult) string {
	unit := strings.TrimSpace(result.Unit)
	if unit == "" {
		unit = "USD"
	}
	return unit
}

func balanceQueryDisplayPlanName(result *dto.BalanceQueryResult) string {
	plan := strings.TrimSpace(result.PlanName)
	if plan == "" {
		plan = "默认套餐"
	}
	return plan
}

func formatProviderBalanceDailyReport(report providerBalanceDailyReport) string {
	if report.Result == nil {
		message := strings.TrimSpace(report.LastError)
		if message == "" && report.QueryError != nil {
			message = report.QueryError.Error()
		}
		if message == "" {
			message = "无结果"
		}
		return "失败：" + message
	}
	if !report.Result.IsValid {
		message := strings.TrimSpace(report.Result.InvalidMessage)
		if message == "" {
			message = strings.TrimSpace(report.LastError)
		}
		if message == "" && report.QueryError != nil {
			message = report.QueryError.Error()
		}
		if message == "" {
			message = "结果无效"
		}
		return "失败：" + message
	}
	dailyUsedText := "暂无基准"
	if report.HasDailyUsed {
		dailyUsedText = fmt.Sprintf("%.4f", report.DailyUsed)
	}
	return fmt.Sprintf(
		"%s 剩余 %.4f %s，已用 %.4f，总量 %.4f，当天使用量 %s",
		balanceQueryDisplayPlanName(report.Result),
		report.Result.Remaining,
		balanceQueryDisplayUnit(report.Result),
		report.Result.Used,
		report.Result.Total,
		dailyUsedText,
	)
}

func buildProviderBalanceDailySummary(reports []providerBalanceDailyReport) string {
	if len(reports) == 0 {
		return ""
	}
	lines := []string{
		fmt.Sprintf("每日供应商余额查询结果：%s", time.Now().Format("2006-01-02")),
		fmt.Sprintf("共查询 %d 个开启余额查询的供应商：", len(reports)),
	}
	for i, report := range reports {
		if i >= dailyProviderBalanceNotifyMaxRows {
			lines = append(lines, fmt.Sprintf("... 还有 %d 个结果未展示", len(reports)-i))
			break
		}
		lines = append(lines, fmt.Sprintf(
			"%d. 供应商ID %d，%s：%s",
			i+1,
			report.ProviderID,
			report.Name,
			formatProviderBalanceDailyReport(report),
		))
	}
	return strings.Join(lines, "\n")
}

func updateConfiguredProviderBalancesDaily() error {
	providers, _, err := model.ListChannelProviders(0, 0)
	if err != nil {
		return err
	}
	reports := make([]providerBalanceDailyReport, 0)
	for _, provider := range providers {
		if provider == nil || provider.Status != common.ChannelStatusEnabled {
			continue
		}
		settings := provider.GetOtherSettings()
		if !settings.BalanceQuery.Enabled {
			continue
		}
		previousDailyResult := cloneBalanceQueryResult(settings.BalanceQuery.LastDailyResult)
		_, err := updateProviderBalance(provider)
		updated, getErr := model.GetChannelProviderByID(provider.Id)
		if getErr != nil {
			common.SysLog(fmt.Sprintf("failed to reload provider after daily balance query: provider_id=%d, error=%v", provider.Id, getErr))
			time.Sleep(common.RequestInterval)
			continue
		}
		newSettings := updated.GetOtherSettings()
		newResult := cloneBalanceQueryResult(newSettings.BalanceQuery.LastResult)
		dailyUsed, hasDailyUsed := calculateBalanceQueryDailyUsed(previousDailyResult, newResult)
		if err == nil && newResult != nil && newResult.IsValid {
			newSettings.BalanceQuery.LastDailyResult = cloneBalanceQueryResult(newResult)
			updated.SetOtherSettings(newSettings)
			if updateErr := model.DB.Model(&model.ChannelProvider{}).Where("id = ?", updated.Id).Updates(map[string]interface{}{
				"settings":     updated.Settings,
				"updated_time": common.GetTimestamp(),
			}).Error; updateErr != nil {
				common.SysLog(fmt.Sprintf("failed to persist provider daily balance snapshot: provider_id=%d, error=%v", provider.Id, updateErr))
			}
		}
		reports = append(reports, providerBalanceDailyReport{
			ProviderID:   provider.Id,
			Name:         provider.Name,
			Result:       newResult,
			DailyUsed:    dailyUsed,
			HasDailyUsed: hasDailyUsed,
			LastError:    newSettings.BalanceQuery.LastError,
			QueryError:   err,
		})
		if err == nil && updated.Balance <= 0 {
			disableProviderChannelsForBalance(provider.Id)
		}
		time.Sleep(common.RequestInterval)
	}
	if message := buildProviderBalanceDailySummary(reports); message != "" {
		sendQQAdminMessage(message, "daily provider balance summary")
	}
	return nil
}

func durationUntilNextMidnight(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return next.Sub(now)
}

func updateConfiguredChannelsBalanceDueLegacy() error {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	executedSharedSources := map[int]struct{}{}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if channel.ChannelInfo.IsMultiKey {
			continue
		}
		if !shouldRunChannelBalanceQuery(channel, now) {
			continue
		}
		settings := channel.GetOtherSettings()
		config := settings.BalanceQuery
		if config.SourceChannelID > 0 && config.SourceChannelID != channel.Id {
			if _, ok := executedSharedSources[config.SourceChannelID]; ok {
				continue
			}
			sourceChannel, sourceErr := model.GetChannelById(config.SourceChannelID, true)
			if sourceErr != nil {
				result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: sourceErr.Error(), CheckedAt: common.GetTimestamp()}
				persistBalanceQueryResult(channel, settings, result, sourceErr)
				continue
			}
			sourceSettings := sourceChannel.GetOtherSettings()
			if !sourceSettings.BalanceQuery.Enabled || sourceSettings.BalanceQuery.SourceChannelID > 0 {
				sourceErr = errors.New("共享余额查询源渠道未启用独立余额查询配置")
				result := dto.BalanceQueryResult{IsValid: false, InvalidMessage: sourceErr.Error(), CheckedAt: common.GetTimestamp()}
				persistBalanceQueryResult(channel, settings, result, sourceErr)
				continue
			}
			balance, result, _, sourceErr := executeChannelConfiguredBalance(sourceChannel, sourceSettings, balanceQueryExecution{Channel: sourceChannel, Settings: sourceSettings})
			executedSharedSources[config.SourceChannelID] = struct{}{}
			propagateSharedBalanceQueryResult(channels, config.SourceChannelID, result, sourceErr)
			if sourceErr == nil {
				sendChannelLowBalanceNotify(sourceChannel, balance)
			}
			if sourceErr == nil && balance <= 0 {
				service.DisableChannel(*types.NewChannelError(sourceChannel.Id, sourceChannel.Type, sourceChannel.Name, sourceChannel.ChannelInfo.IsMultiKey, "", sourceChannel.GetAutoBan()), "余额不足")
			}
			time.Sleep(common.RequestInterval)
			continue
		}
		balance, _, configured, err := executeChannelConfiguredBalance(channel, settings, balanceQueryExecution{Channel: channel, Settings: settings})
		if !configured {
			continue
		}
		executedSharedSources[channel.Id] = struct{}{}
		if err == nil {
			lastResult := channel.GetOtherSettings().BalanceQuery.LastResult
			propagateSharedBalanceQueryResult(channels, channel.Id, lastResult, nil)
			sendChannelLowBalanceNotify(channel, balance)
		}
		if err == nil && balance <= 0 {
			service.DisableChannel(*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan()), "余额不足")
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func disableProviderChannelsForBalance(providerID int) {
	var channels []*model.Channel
	if err := model.DB.Where("provider_id = ? AND status = ?", providerID, common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		common.SysLog(fmt.Sprintf("failed to list provider channels for balance disable: provider_id=%d, error=%v", providerID, err))
		return
	}
	for _, channel := range channels {
		service.DisableChannel(*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan()), "供应商余额不足")
	}
}

func propagateSharedBalanceQueryResult(channels []*model.Channel, sourceChannelID int, result *dto.BalanceQueryResult, queryErr error) {
	if result == nil {
		return
	}
	for _, channel := range channels {
		if channel.Id == sourceChannelID {
			continue
		}
		settings := channel.GetOtherSettings()
		if !settings.BalanceQuery.Enabled || settings.BalanceQuery.SourceChannelID != sourceChannelID {
			continue
		}
		persistBalanceQueryResult(channel, settings, *result, queryErr)
	}
}

func updateAllChannelsBalance() error {
	providers, _, err := model.ListChannelProviders(0, 0)
	if err != nil {
		return err
	}
	for _, provider := range providers {
		if provider.Status != common.ChannelStatusEnabled {
			continue
		}
		balance, err := updateProviderBalance(provider)
		if err == nil && balance <= 0 {
			disableProviderChannelsForBalance(provider.Id)
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func updateAllChannelsBalanceLegacy() error {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if channel.ChannelInfo.IsMultiKey {
			continue // skip multi-key channels
		}
		// TODO: support Azure
		//if channel.Type != common.ChannelTypeOpenAI && channel.Type != common.ChannelTypeCustom {
		//	continue
		//}
		balance, err := updateChannelBalance(channel)
		if err != nil {
			continue
		} else {
			// err is nil & balance <= 0 means quota is used up
			if balance <= 0 {
				service.DisableChannel(*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan()), "余额不足")
			}
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func UpdateAllChannelsBalance(c *gin.Context) {
	// TODO: make it async
	err := updateAllChannelsBalance()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func AutomaticallyUpdateChannels(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		common.SysLog("updating all channels")
		_ = updateAllChannelsBalance()
		common.SysLog("channels update done")
	}
}

func StartChannelBalanceQueryTask() {
	channelBalanceQueryTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		go func() {
			common.SysLog("channel balance query task started")
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := updateConfiguredChannelsBalanceDue(); err != nil {
					common.SysLog("channel balance query task failed: " + err.Error())
				}
			}
		}()
		go func() {
			common.SysLog("daily provider balance query task started")
			for {
				timer := time.NewTimer(durationUntilNextMidnight(time.Now()))
				<-timer.C
				common.SysLog("daily provider balance query started")
				if err := updateConfiguredProviderBalancesDaily(); err != nil {
					common.SysLog("daily provider balance query failed: " + err.Error())
				}
				common.SysLog("daily provider balance query done")
			}
		}()
	})
}
