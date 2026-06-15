package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	groupQueryDefaultIntervalSecond = 300
	groupQueryNotifyMaxChanges      = 12
	groupRatioSyncEpsilon           = 1e-9
)

var channelGroupQueryTaskOnce sync.Once

type groupQueryExecution struct {
	Channel  *model.Channel
	Settings dto.ChannelOtherSettings
}

type providerGroupQueryExecution struct {
	Provider *model.ChannelProvider
	Channel  *model.Channel
	Settings dto.ChannelProviderSettings
}

type groupQueryDebugInfo struct {
	ChannelID int               `json:"channel_id"`
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body,omitempty"`
	Response  string            `json:"response,omitempty"`
	Error     string            `json:"error,omitempty"`
}

type groupQuerySourceItem struct {
	SourceType      string                        `json:"source_type"`
	ID              int                           `json:"id"`
	Name            string                        `json:"name"`
	Type            int                           `json:"type,omitempty"`
	Template        string                        `json:"template,omitempty"`
	IntervalSeconds int                           `json:"interval_seconds"`
	LastCheckTime   int64                         `json:"last_check_time"`
	LastError       string                        `json:"last_error,omitempty"`
	LastResult      map[string]dto.GroupQueryItem `json:"last_result,omitempty"`
}

func getGroupQueryIntervalSeconds(config dto.GroupQuery) int {
	if config.IntervalSeconds == nil {
		return groupQueryDefaultIntervalSecond
	}
	if *config.IntervalSeconds < 0 {
		return groupQueryDefaultIntervalSecond
	}
	return *config.IntervalSeconds
}

func buildGroupQueryConfig(channel *model.Channel, config dto.GroupQuery) dto.GroupQuery {
	template := normalizeBalanceQueryTemplate(config.Template)
	switch template {
	case balanceQueryTemplateNewAPI, balanceQueryTemplateSub2API:
		config.Template = template
	default:
		return config
	}

	if template == balanceQueryTemplateSub2API {
		if strings.TrimSpace(config.Request.URL) == "" {
			config.Request.URL = "{{baseUrl}}/api/v1/groups/available?timezone=Asia%2FShanghai"
		}
		if strings.TrimSpace(config.Request.Method) == "" {
			config.Request.Method = http.MethodGet
		}
		if config.Request.Headers == nil {
			config.Request.Headers = map[string]string{}
		}
		if _, ok := config.Request.Headers["Authorization"]; !ok {
			config.Request.Headers["Authorization"] = "Bearer {{accessToken}}"
		}
		if strings.TrimSpace(config.Extractor.DataPath) == "" {
			config.Extractor.DataPath = "data"
		}
		if strings.TrimSpace(config.Extractor.DescPath) == "" {
			config.Extractor.DescPath = "description"
		}
		if strings.TrimSpace(config.Extractor.RatioPath) == "" {
			config.Extractor.RatioPath = "rate_multiplier"
		}
		if strings.TrimSpace(config.Extractor.SuccessPath) == "" {
			config.Extractor.SuccessPath = "code"
		}
		if strings.TrimSpace(config.Extractor.SuccessValue) == "" {
			config.Extractor.SuccessValue = "0"
		}
		config.Extractor.SuccessOptional = false
		if strings.TrimSpace(config.Extractor.MessagePath) == "" {
			config.Extractor.MessagePath = "message"
		}
		_ = channel
		return config
	}

	if strings.TrimSpace(config.Request.URL) == "" {
		config.Request.URL = "{{baseUrl}}/api/user/self/groups"
	}
	if strings.TrimSpace(config.Request.Method) == "" {
		config.Request.Method = http.MethodGet
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
	if strings.TrimSpace(config.Extractor.DataPath) == "" {
		config.Extractor.DataPath = "data"
	}
	if strings.TrimSpace(config.Extractor.DescPath) == "" {
		config.Extractor.DescPath = "desc"
	}
	if strings.TrimSpace(config.Extractor.RatioPath) == "" {
		config.Extractor.RatioPath = "ratio"
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

func extractGroupQueryArrayResult(data gjson.Result, extractor dto.GroupQueryExtractorConfig) (map[string]dto.GroupQueryItem, error) {
	result := make(map[string]dto.GroupQueryItem)
	var parseErr error
	data.ForEach(func(_, value gjson.Result) bool {
		if !value.IsObject() {
			parseErr = errors.New("上游分组数据格式无效")
			return false
		}
		groupName := strings.TrimSpace(value.Get("name").String())
		if groupName == "" {
			groupName = strings.TrimSpace(value.Get("group").String())
		}
		if groupName == "" {
			groupName = strings.TrimSpace(value.Get("id").String())
		}
		if groupName == "" {
			parseErr = errors.New("上游分组缺少名称字段")
			return false
		}
		item := dto.GroupQueryItem{Desc: groupName}
		if extractor.DescPath != "" {
			if desc := strings.TrimSpace(value.Get(extractor.DescPath).String()); desc != "" {
				item.Desc = desc
			}
		}
		ratio := value.Get(extractor.RatioPath)
		if !ratio.Exists() || ratio.Type == gjson.Null {
			parseErr = fmt.Errorf("上游分组 %s 缺少倍率字段", groupName)
			return false
		}
		item.Ratio = ratio.Float()
		result[groupName] = item
		return true
	})
	if parseErr != nil {
		return nil, parseErr
	}
	if len(result) == 0 {
		return nil, errors.New("上游分组查询响应没有可用分组")
	}
	return result, nil
}

func replaceGroupQueryVars(value string, channel *model.Channel, config dto.GroupQuery) string {
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

func logGroupQueryDebug(info groupQueryDebugInfo) {
	data, err := common.Marshal(info)
	if err != nil {
		common.SysLog(fmt.Sprintf("group query debug: channel_id=%d, marshal error=%v", info.ChannelID, err))
		return
	}
	common.SysLog("group query debug: " + string(data))
}

func validateGroupQuerySuccess(body []byte, extractor dto.GroupQueryExtractorConfig) (bool, string) {
	if extractor.SuccessPath == "" {
		return true, ""
	}
	result, ok := getBalanceQueryJSONValue(body, extractor.SuccessPath)
	if !ok {
		if extractor.SuccessOptional {
			return true, ""
		}
		return false, "上游分组查询响应缺少成功状态字段"
	}
	expected := strings.TrimSpace(extractor.SuccessValue)
	if expected == "" {
		if result.Type == gjson.True {
			return true, ""
		}
		return false, "上游分组查询响应状态为失败"
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
	return false, "上游分组查询响应状态为失败"
}

func extractGroupQueryResult(body []byte, extractor dto.GroupQueryExtractorConfig) (map[string]dto.GroupQueryItem, error) {
	if ok, message := validateGroupQuerySuccess(body, extractor); !ok {
		if extractor.MessagePath != "" {
			if upstreamMessage := strings.TrimSpace(gjson.GetBytes(body, extractor.MessagePath).String()); upstreamMessage != "" {
				message = upstreamMessage
			}
		}
		return nil, errors.New(message)
	}
	data := gjson.ParseBytes(body)
	if extractor.DataPath != "" {
		value, ok := getBalanceQueryJSONValue(body, extractor.DataPath)
		if !ok {
			return nil, errors.New("上游分组查询响应缺少分组数据字段")
		}
		data = value
	}
	if data.IsArray() {
		return extractGroupQueryArrayResult(data, extractor)
	}
	if !data.IsObject() {
		return nil, errors.New("上游分组查询响应分组数据不是对象")
	}
	result := make(map[string]dto.GroupQueryItem)
	var parseErr error
	data.ForEach(func(key, value gjson.Result) bool {
		groupName := strings.TrimSpace(key.String())
		if groupName == "" {
			return true
		}
		item := dto.GroupQueryItem{Desc: groupName}
		if value.IsObject() {
			if extractor.DescPath != "" {
				if desc := strings.TrimSpace(value.Get(extractor.DescPath).String()); desc != "" {
					item.Desc = desc
				}
			}
			ratio := value.Get(extractor.RatioPath)
			if !ratio.Exists() || ratio.Type == gjson.Null {
				parseErr = fmt.Errorf("上游分组 %s 缺少倍率字段", groupName)
				return false
			}
			item.Ratio = ratio.Float()
		} else if value.Type == gjson.Number {
			item.Ratio = value.Float()
		} else {
			parseErr = fmt.Errorf("上游分组 %s 数据格式无效", groupName)
			return false
		}
		result[groupName] = item
		return true
	})
	if parseErr != nil {
		return nil, parseErr
	}
	if len(result) == 0 {
		return nil, errors.New("上游分组查询响应没有可用分组")
	}
	return result, nil
}

func groupRatioSyncNearlyEqual(a, b float64) bool {
	if a > b {
		return a-b < groupRatioSyncEpsilon
	}
	return b-a < groupRatioSyncEpsilon
}

func applyUpstreamGroupRatioBindingsToMap(
	sourceType string,
	sourceID int,
	result map[string]dto.GroupQueryItem,
	bindings map[string]ratio_setting.UpstreamGroupRatioBinding,
	currentRatios map[string]float64,
) (map[string]float64, bool) {
	nextRatios := make(map[string]float64, len(currentRatios))
	for group, ratio := range currentRatios {
		nextRatios[group] = ratio
	}

	changed := false
	for localGroup, binding := range bindings {
		if binding.SourceType != sourceType || binding.SourceID != sourceID {
			continue
		}
		currentRatio, exists := currentRatios[localGroup]
		if !exists {
			common.SysLog(fmt.Sprintf("skip upstream group ratio sync: local_group=%s source_type=%s source_id=%d reason=local group missing", localGroup, sourceType, sourceID))
			continue
		}
		item, ok := result[binding.UpstreamGroup]
		if !ok {
			common.SysLog(fmt.Sprintf("skip upstream group ratio sync: local_group=%s source_type=%s source_id=%d upstream_group=%s reason=upstream group missing", localGroup, sourceType, sourceID, binding.UpstreamGroup))
			continue
		}
		nextRatio, err := ratio_setting.CalculateUpstreamGroupBoundRatio(item.Ratio, binding)
		if err != nil {
			common.SysLog(fmt.Sprintf("skip upstream group ratio sync: local_group=%s source_type=%s source_id=%d upstream_group=%s reason=invalid ratio: %s", localGroup, sourceType, sourceID, binding.UpstreamGroup, err.Error()))
			continue
		}
		if groupRatioSyncNearlyEqual(currentRatio, nextRatio) {
			continue
		}
		nextRatios[localGroup] = nextRatio
		changed = true
	}
	return nextRatios, changed
}

func syncUpstreamGroupRatioBindings(sourceType string, sourceID int, result map[string]dto.GroupQueryItem) {
	if sourceID <= 0 || len(result) == 0 {
		return
	}

	bindings := ratio_setting.GetUpstreamGroupRatioBindingsCopy()
	if len(bindings) == 0 {
		return
	}

	nextRatios, changed := applyUpstreamGroupRatioBindingsToMap(
		sourceType,
		sourceID,
		result,
		bindings,
		ratio_setting.GetGroupRatioCopy(),
	)

	if !changed {
		return
	}

	data, err := common.Marshal(nextRatios)
	if err != nil {
		common.SysLog("failed to marshal synced group ratios: " + err.Error())
		return
	}
	if err := model.UpdateGroupRatioOptionWithSource(string(data), model.GroupRatioHistorySourceSync); err != nil {
		common.SysLog("failed to persist synced group ratios: " + err.Error())
	}
}

func syncUpstreamGroupRatioBindingsFromLastResults() error {
	providers, _, err := model.ListChannelProviders(0, 0)
	if err != nil {
		return err
	}
	for _, provider := range providers {
		settings := provider.GetOtherSettings()
		if !settings.GroupQuery.Enabled || len(settings.GroupQuery.LastResult) == 0 {
			continue
		}
		syncUpstreamGroupRatioBindings(
			ratio_setting.UpstreamGroupRatioBindingSourceProvider,
			provider.Id,
			settings.GroupQuery.LastResult,
		)
	}

	channels, err := model.GetAllChannels(0, 0, true, true)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		settings := channel.GetOtherSettings()
		if !settings.GroupQuery.Enabled || len(settings.GroupQuery.LastResult) == 0 {
			continue
		}
		syncUpstreamGroupRatioBindings(
			ratio_setting.UpstreamGroupRatioBindingSourceChannel,
			channel.Id,
			settings.GroupQuery.LastResult,
		)
	}
	return nil
}

func persistGroupQueryResult(channel *model.Channel, settings dto.ChannelOtherSettings, result map[string]dto.GroupQueryItem, queryErr error) {
	settings.GroupQuery.LastCheckTime = common.GetTimestamp()
	if queryErr != nil {
		settings.GroupQuery.LastError = queryErr.Error()
	} else {
		settings.GroupQuery.LastResult = result
		settings.GroupQuery.LastError = ""
	}
	channel.SetOtherSettings(settings)
	if updateErr := model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(map[string]interface{}{
		"settings": channel.OtherSettings,
	}).Error; updateErr != nil {
		common.SysLog(fmt.Sprintf("failed to persist group query result: channel_id=%d, error=%v", channel.Id, updateErr))
	}
	if queryErr == nil {
		syncUpstreamGroupRatioBindings(ratio_setting.UpstreamGroupRatioBindingSourceChannel, channel.Id, result)
	}
}

func persistProviderGroupQueryResult(provider *model.ChannelProvider, settings dto.ChannelProviderSettings, result map[string]dto.GroupQueryItem, queryErr error) {
	settings.GroupQuery.LastCheckTime = common.GetTimestamp()
	if queryErr != nil {
		settings.GroupQuery.LastError = queryErr.Error()
	} else {
		settings.GroupQuery.LastResult = result
		settings.GroupQuery.LastError = ""
	}
	provider.SetOtherSettings(settings)
	if updateErr := model.DB.Model(&model.ChannelProvider{}).Where("id = ?", provider.Id).Updates(map[string]interface{}{
		"settings":     provider.Settings,
		"updated_time": common.GetTimestamp(),
	}).Error; updateErr != nil {
		common.SysLog(fmt.Sprintf("failed to persist provider group query result: provider_id=%d, error=%v", provider.Id, updateErr))
	}
	if queryErr == nil {
		syncUpstreamGroupRatioBindings(ratio_setting.UpstreamGroupRatioBindingSourceProvider, provider.Id, result)
	}
}

func updateChannelConfiguredGroups(channel *model.Channel) (map[string]dto.GroupQueryItem, bool, error) {
	settings := channel.GetOtherSettings()
	config := settings.GroupQuery
	if !config.Enabled {
		return nil, false, nil
	}
	execution := groupQueryExecution{Channel: channel, Settings: settings}
	if config.SourceChannelID > 0 && config.SourceChannelID != channel.Id {
		sourceChannel, err := model.GetChannelById(config.SourceChannelID, true)
		if err != nil {
			persistGroupQueryResult(channel, settings, nil, err)
			return nil, true, err
		}
		sourceSettings := sourceChannel.GetOtherSettings()
		if !sourceSettings.GroupQuery.Enabled || sourceSettings.GroupQuery.SourceChannelID > 0 {
			err = errors.New("共享上游分组查询源渠道未启用独立分组查询配置")
			persistGroupQueryResult(channel, settings, nil, err)
			return nil, true, err
		}
		execution = groupQueryExecution{Channel: sourceChannel, Settings: sourceSettings}
	}
	return executeChannelConfiguredGroups(channel, settings, execution)
}

func updateProviderConfiguredGroups(provider *model.ChannelProvider) (map[string]dto.GroupQueryItem, bool, error) {
	settings := provider.GetOtherSettings()
	config := settings.GroupQuery
	if !config.Enabled {
		return nil, false, nil
	}
	sourceChannel, err := getProviderQuerySourceChannel(provider, config.SourceChannelID)
	if err != nil {
		persistProviderGroupQueryResult(provider, settings, nil, err)
		return nil, true, err
	}
	return executeProviderConfiguredGroups(provider, settings, providerGroupQueryExecution{Provider: provider, Channel: sourceChannel, Settings: settings})
}

func executeProviderConfiguredGroups(provider *model.ChannelProvider, providerSettings dto.ChannelProviderSettings, execution providerGroupQueryExecution) (map[string]dto.GroupQueryItem, bool, error) {
	channel := execution.Channel
	config := buildGroupQueryConfig(channel, execution.Settings.GroupQuery)
	method := strings.ToUpper(strings.TrimSpace(config.Request.Method))
	if method == "" {
		method = http.MethodGet
	}
	url := replaceGroupQueryVars(config.Request.URL, channel, config)
	if strings.TrimSpace(url) == "" {
		err := errors.New("上游分组查询请求地址不能为空")
		persistProviderGroupQueryResult(provider, providerSettings, nil, err)
		return nil, true, err
	}
	headers := http.Header{}
	for key, value := range config.Request.Headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		headers.Set(key, replaceGroupQueryVars(value, channel, config))
	}
	requestBody := replaceGroupQueryVars(config.Request.Body, channel, config)
	debugInfo := groupQueryDebugInfo{
		ChannelID: channel.Id,
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
				providerSettings = updateProviderGroupQueryTokens(provider, providerSettings, tokens)
				config.AccessToken = tokens.AccessToken
				config.RefreshToken = tokens.RefreshToken
				headers = http.Header{}
				for key, value := range config.Request.Headers {
					if strings.TrimSpace(key) == "" {
						continue
					}
					headers.Set(key, replaceGroupQueryVars(value, channel, config))
				}
				requestBody = replaceGroupQueryVars(config.Request.Body, channel, config)
				debugInfo.Headers = balanceQueryHeadersToMap(headers)
				debugInfo.Body = requestBody
				body, err = GetResponseBodyWithBody(method, url, requestBody, channel, headers)
			} else {
				err = fmt.Errorf("上游分组查询 401 后刷新 token 失败: %w", refreshErr)
			}
		}
	}
	if err != nil {
		debugInfo.Error = err.Error()
		logGroupQueryDebug(debugInfo)
		persistProviderGroupQueryResult(provider, providerSettings, nil, err)
		return nil, true, err
	}
	debugInfo.Response = string(body)
	result, err := extractGroupQueryResult(body, config.Extractor)
	if err != nil {
		debugInfo.Error = err.Error()
		logGroupQueryDebug(debugInfo)
		persistProviderGroupQueryResult(provider, providerSettings, nil, err)
		return nil, true, err
	}
	previous := providerSettings.GroupQuery.LastResult
	persistProviderGroupQueryResult(provider, providerSettings, result, nil)
	if previous != nil && !reflect.DeepEqual(previous, result) {
		sendProviderGroupQueryChangedNotify(provider, previous, result)
	}
	return result, true, nil
}

func executeChannelConfiguredGroups(target *model.Channel, targetSettings dto.ChannelOtherSettings, execution groupQueryExecution) (map[string]dto.GroupQueryItem, bool, error) {
	channel := execution.Channel
	settings := execution.Settings
	config := buildGroupQueryConfig(channel, settings.GroupQuery)
	method := strings.ToUpper(strings.TrimSpace(config.Request.Method))
	if method == "" {
		method = http.MethodGet
	}
	url := replaceGroupQueryVars(config.Request.URL, channel, config)
	if strings.TrimSpace(url) == "" {
		err := errors.New("上游分组查询请求地址不能为空")
		persistGroupQueryResult(target, targetSettings, nil, err)
		return nil, true, err
	}
	headers := http.Header{}
	for key, value := range config.Request.Headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		headers.Set(key, replaceGroupQueryVars(value, channel, config))
	}
	requestBody := replaceGroupQueryVars(config.Request.Body, channel, config)
	debugInfo := groupQueryDebugInfo{
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
				settings = updateChannelGroupQueryTokens(channel, settings, tokens)
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
					headers.Set(key, replaceGroupQueryVars(value, channel, config))
				}
				requestBody = replaceGroupQueryVars(config.Request.Body, channel, config)
				debugInfo.Headers = balanceQueryHeadersToMap(headers)
				debugInfo.Body = requestBody
				body, err = GetResponseBodyWithBody(method, url, requestBody, channel, headers)
			} else {
				err = fmt.Errorf("上游分组查询 401 后刷新 token 失败: %w", refreshErr)
			}
		}
	}
	if err != nil {
		debugInfo.Error = err.Error()
		logGroupQueryDebug(debugInfo)
		persistGroupQueryResult(target, targetSettings, nil, err)
		return nil, true, err
	}
	debugInfo.Response = string(body)
	result, err := extractGroupQueryResult(body, config.Extractor)
	if err != nil {
		debugInfo.Error = err.Error()
		logGroupQueryDebug(debugInfo)
		persistGroupQueryResult(target, targetSettings, nil, err)
		return nil, true, err
	}
	previous := targetSettings.GroupQuery.LastResult
	persistGroupQueryResult(target, targetSettings, result, nil)
	if previous != nil && !reflect.DeepEqual(previous, result) {
		sendChannelGroupQueryChangedNotify(target, previous, result)
	}
	return result, true, nil
}

func shouldRunChannelGroupQuery(channel *model.Channel, now int64) bool {
	settings := channel.GetOtherSettings()
	config := settings.GroupQuery
	if !config.Enabled {
		return false
	}
	interval := getGroupQueryIntervalSeconds(config)
	if interval == 0 {
		return false
	}
	return config.LastCheckTime <= 0 || now-config.LastCheckTime >= int64(interval)
}

func shouldRunProviderGroupQuery(provider *model.ChannelProvider, now int64) bool {
	settings := provider.GetOtherSettings()
	config := settings.GroupQuery
	if !config.Enabled {
		return false
	}
	interval := getGroupQueryIntervalSeconds(config)
	if interval == 0 {
		return false
	}
	return config.LastCheckTime <= 0 || now-config.LastCheckTime >= int64(interval)
}

func propagateSharedGroupQueryResult(channels []*model.Channel, sourceChannelID int, result map[string]dto.GroupQueryItem, queryErr error) {
	if result == nil && queryErr == nil {
		return
	}
	for _, channel := range channels {
		if channel.Id == sourceChannelID {
			continue
		}
		settings := channel.GetOtherSettings()
		if !settings.GroupQuery.Enabled || settings.GroupQuery.SourceChannelID != sourceChannelID {
			continue
		}
		persistGroupQueryResult(channel, settings, result, queryErr)
	}
}

func updateConfiguredChannelsGroupsDue() error {
	providers, _, err := model.ListChannelProviders(0, 0)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	for _, provider := range providers {
		if provider.Status != common.ChannelStatusEnabled {
			continue
		}
		if !shouldRunProviderGroupQuery(provider, now) {
			continue
		}
		_, configured, _ := updateProviderConfiguredGroups(provider)
		if !configured {
			continue
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func updateConfiguredChannelsGroupsDueLegacy() error {
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
		if !shouldRunChannelGroupQuery(channel, now) {
			continue
		}
		settings := channel.GetOtherSettings()
		config := settings.GroupQuery
		if config.SourceChannelID > 0 && config.SourceChannelID != channel.Id {
			if _, ok := executedSharedSources[config.SourceChannelID]; ok {
				continue
			}
			sourceChannel, sourceErr := model.GetChannelById(config.SourceChannelID, true)
			if sourceErr != nil {
				persistGroupQueryResult(channel, settings, nil, sourceErr)
				continue
			}
			sourceSettings := sourceChannel.GetOtherSettings()
			if !sourceSettings.GroupQuery.Enabled || sourceSettings.GroupQuery.SourceChannelID > 0 {
				sourceErr = errors.New("共享上游分组查询源渠道未启用独立分组查询配置")
				persistGroupQueryResult(channel, settings, nil, sourceErr)
				continue
			}
			result, _, sourceErr := executeChannelConfiguredGroups(sourceChannel, sourceSettings, groupQueryExecution{Channel: sourceChannel, Settings: sourceSettings})
			executedSharedSources[config.SourceChannelID] = struct{}{}
			propagateSharedGroupQueryResult(channels, config.SourceChannelID, result, sourceErr)
			time.Sleep(common.RequestInterval)
			continue
		}
		result, configured, err := executeChannelConfiguredGroups(channel, settings, groupQueryExecution{Channel: channel, Settings: settings})
		if !configured {
			continue
		}
		executedSharedSources[channel.Id] = struct{}{}
		if err == nil {
			propagateSharedGroupQueryResult(channels, channel.Id, result, nil)
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func updateAllChannelsGroups() error {
	providers, _, err := model.ListChannelProviders(0, 0)
	if err != nil {
		return err
	}
	for _, provider := range providers {
		if provider.Status != common.ChannelStatusEnabled {
			continue
		}
		_, configured, _ := updateProviderConfiguredGroups(provider)
		if !configured {
			continue
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func updateAllChannelsGroupsLegacy() error {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if channel.ChannelInfo.IsMultiKey {
			continue
		}
		_, configured, err := updateChannelConfiguredGroups(channel)
		if !configured || err != nil {
			continue
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

func formatGroupQueryChanges(previous, current map[string]dto.GroupQueryItem) []string {
	keys := map[string]struct{}{}
	for key := range previous {
		keys[key] = struct{}{}
	}
	for key := range current {
		keys[key] = struct{}{}
	}
	names := make([]string, 0, len(keys))
	for key := range keys {
		names = append(names, key)
	}
	sort.Strings(names)

	changes := make([]string, 0)
	for _, key := range names {
		oldItem, oldOK := previous[key]
		newItem, newOK := current[key]
		switch {
		case !oldOK && newOK:
			changes = append(changes, fmt.Sprintf("+ %s(desc=%s, ratio=%g)", key, newItem.Desc, newItem.Ratio))
		case oldOK && !newOK:
			changes = append(changes, fmt.Sprintf("- %s(desc=%s, ratio=%g)", key, oldItem.Desc, oldItem.Ratio))
		case oldOK && newOK && !reflect.DeepEqual(oldItem, newItem):
			changes = append(changes, fmt.Sprintf("* %s(desc: %s -> %s, ratio: %g -> %g)", key, oldItem.Desc, newItem.Desc, oldItem.Ratio, newItem.Ratio))
		}
		if len(changes) >= groupQueryNotifyMaxChanges {
			changes = append(changes, "...")
			break
		}
	}
	return changes
}

func sendChannelGroupQueryChangedNotify(channel *model.Channel, previous, current map[string]dto.GroupQueryItem) {
	if channel == nil || common.QQCallbackAddress == "" || common.QQCallbackAccessToken == "" || common.QQAdminNumber == "" {
		return
	}
	changes := formatGroupQueryChanges(previous, current)
	if len(changes) == 0 {
		return
	}
	go func() {
		message := fmt.Sprintf("渠道上游分组发生变化：渠道ID %d，名称 %s\n%s", channel.Id, channel.Name, strings.Join(changes, "\n"))
		payload, err := common.Marshal(channelLowBalanceNotifyPayload{
			QQ:      common.QQAdminNumber,
			AdminQQ: common.QQAdminNumber,
			To:      common.QQAdminNumber,
			Message: message,
			Content: message,
		})
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to marshal channel group query notify payload: channel_id=%d, error=%v", channel.Id, err))
			return
		}
		req, err := http.NewRequest(http.MethodPost, channelQQServiceURL("/api/nachoai/send_message"), bytes.NewReader(payload))
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to create channel group query notify request: channel_id=%d, error=%v", channel.Id, err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		setChannelQQServiceAuthHeader(req)
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to send channel group query notify: channel_id=%d, error=%v", channel.Id, err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			common.SysLog(fmt.Sprintf("failed to send channel group query notify: channel_id=%d, status=%d", channel.Id, resp.StatusCode))
		}
	}()
}

func sendProviderGroupQueryChangedNotify(provider *model.ChannelProvider, previous, current map[string]dto.GroupQueryItem) {
	if provider == nil || common.QQCallbackAddress == "" || common.QQCallbackAccessToken == "" || common.QQAdminNumber == "" {
		return
	}
	changes := formatGroupQueryChanges(previous, current)
	if len(changes) == 0 {
		return
	}
	go func() {
		message := fmt.Sprintf("供应商上游分组发生变化：供应商ID %d，名称 %s\n%s", provider.Id, provider.Name, strings.Join(changes, "\n"))
		payload, err := common.Marshal(channelLowBalanceNotifyPayload{
			QQ:      common.QQAdminNumber,
			AdminQQ: common.QQAdminNumber,
			To:      common.QQAdminNumber,
			Message: message,
			Content: message,
		})
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to marshal provider group query notify payload: provider_id=%d, error=%v", provider.Id, err))
			return
		}
		req, err := http.NewRequest(http.MethodPost, channelQQServiceURL("/api/nachoai/send_message"), bytes.NewReader(payload))
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to create provider group query notify request: provider_id=%d, error=%v", provider.Id, err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		setChannelQQServiceAuthHeader(req)
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to send provider group query notify: provider_id=%d, error=%v", provider.Id, err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			common.SysLog(fmt.Sprintf("failed to send provider group query notify: provider_id=%d, status=%d", provider.Id, resp.StatusCode))
		}
	}()
}

func UpdateChannelGroups(c *gin.Context) {
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
			"message": "多密钥渠道不支持上游分组查询",
		})
		return
	}
	result, configured, err := updateChannelConfiguredGroups(channel)
	if !configured {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道未启用上游分组查询配置",
		})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"groups":                result,
			"settings":              channel.OtherSettings,
			"group_last_check_time": channel.GetOtherSettings().GroupQuery.LastCheckTime,
		},
	})
}

func UpdateChannelProviderGroups(c *gin.Context) {
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
	result, configured, err := updateProviderConfiguredGroups(provider)
	if !configured {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "供应商未启用上游分组查询配置",
		})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"groups":                result,
			"settings":              provider.Settings,
			"group_last_check_time": provider.GetOtherSettings().GroupQuery.LastCheckTime,
		},
	})
}

func UpdateAllChannelsGroups(c *gin.Context) {
	if err := updateAllChannelsGroups(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func GetChannelGroupQueryInstances(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    []gin.H{},
	})
}

func GetChannelGroupQuerySources(c *gin.Context) {
	providers, _, err := model.ListChannelProviders(0, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	sources := make([]groupQuerySourceItem, 0)
	for _, provider := range providers {
		settings := provider.GetOtherSettings()
		config := settings.GroupQuery
		if !config.Enabled {
			continue
		}
		sources = append(sources, groupQuerySourceItem{
			SourceType:      ratio_setting.UpstreamGroupRatioBindingSourceProvider,
			ID:              provider.Id,
			Name:            provider.Name,
			Template:        config.Template,
			IntervalSeconds: getGroupQueryIntervalSeconds(config),
			LastCheckTime:   config.LastCheckTime,
			LastError:       config.LastError,
			LastResult:      config.LastResult,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    sources,
	})
}

func StartChannelGroupQueryTask() {
	channelGroupQueryTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		go func() {
			common.SysLog("channel group query task started")
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := updateConfiguredChannelsGroupsDue(); err != nil {
					common.SysLog("channel group query task failed: " + err.Error())
				}
			}
		}()
	})
}
