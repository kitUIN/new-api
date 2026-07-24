package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"

	"github.com/gin-gonic/gin"
)

type testResult struct {
	context      *gin.Context
	localErr     error
	newAPIError  *types.NewAPIError
	responseBody []byte
}

type channelTestOptions struct {
	request          dto.Request
	skipPricing      bool
	recordPerfMetric bool
	usageLogLabel    string
	usageLogModel    string
}

const defaultChannelTestStream = true

func parseChannelTestStream(c *gin.Context) bool {
	rawStream, ok := c.GetQuery("stream")
	if !ok {
		return defaultChannelTestStream
	}
	rawStream = strings.TrimSpace(rawStream)
	if rawStream == "" {
		return defaultChannelTestStream
	}
	isStream, err := strconv.ParseBool(rawStream)
	if err != nil {
		return defaultChannelTestStream
	}
	return isStream
}

func normalizeChannelTestEndpoint(channel *model.Channel, modelName, endpointType string) string {
	normalized := strings.TrimSpace(endpointType)
	if strings.HasSuffix(modelName, ratio_setting.CompactModelSuffix) {
		return string(constant.EndpointTypeOpenAIResponseCompact)
	}
	if channel != nil && channel.Type == constant.ChannelTypeOpenAI {
		if normalized == "" || normalized == string(constant.EndpointTypeOpenAI) {
			return string(constant.EndpointTypeOpenAIResponse)
		}
	}
	if normalized != "" {
		return normalized
	}
	if channel != nil && channel.Type == constant.ChannelTypeCodex {
		return string(constant.EndpointTypeOpenAIResponse)
	}
	return normalized
}

func normalizeChannelTestModelName(channel *model.Channel, testModel string) string {
	testModel = strings.TrimSpace(testModel)
	if channel == nil || testModel == "" {
		return testModel
	}

	models := channel.GetModels()
	if common.StringsContains(models, testModel) {
		return testModel
	}

	lookupModel := testModel
	hasCompactSuffix := strings.HasSuffix(lookupModel, ratio_setting.CompactModelSuffix)
	if hasCompactSuffix {
		lookupModel = strings.TrimSuffix(lookupModel, ratio_setting.CompactModelSuffix)
	}

	modelMapping := strings.TrimSpace(channel.GetModelMapping())
	if modelMapping == "" || modelMapping == "{}" {
		return testModel
	}

	modelMap := make(map[string]string)
	if err := common.UnmarshalJsonStr(modelMapping, &modelMap); err != nil {
		return testModel
	}

	candidateSources := make([]string, 0, len(models)+len(modelMap))
	seenSources := make(map[string]struct{}, len(models)+len(modelMap))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		candidateSources = append(candidateSources, modelName)
		seenSources[modelName] = struct{}{}
	}
	for source := range modelMap {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if _, ok := seenSources[source]; ok {
			continue
		}
		candidateSources = append(candidateSources, source)
		seenSources[source] = struct{}{}
	}

	for _, sourceModel := range candidateSources {
		for _, mappedModel := range resolveChannelTestModelMappingChain(sourceModel, modelMap) {
			if mappedModel != lookupModel {
				continue
			}
			if hasCompactSuffix {
				return ratio_setting.WithCompactModelSuffix(sourceModel)
			}
			return sourceModel
		}
	}

	return testModel
}

func resolveChannelTestModelMappingChain(sourceModel string, modelMap map[string]string) []string {
	sourceModel = strings.TrimSpace(sourceModel)
	if sourceModel == "" {
		return nil
	}

	currentModel := sourceModel
	visitedModels := map[string]bool{currentModel: true}
	mappedModels := make([]string, 0)
	for {
		mappedModel := strings.TrimSpace(modelMap[currentModel])
		if mappedModel == "" {
			return mappedModels
		}
		mappedModels = append(mappedModels, mappedModel)
		if visitedModels[mappedModel] {
			return mappedModels
		}
		visitedModels[mappedModel] = true
		currentModel = mappedModel
	}
}

func testChannel(channel *model.Channel, testModel string, endpointType string, isStream bool) testResult {
	return testChannelWithOptions(channel, testModel, endpointType, isStream, "", channelTestOptions{recordPerfMetric: true})
}

func testChannelWithGroup(channel *model.Channel, testModel string, endpointType string, isStream bool, testGroup string) (result testResult) {
	return testChannelWithOptions(channel, testModel, endpointType, isStream, testGroup, channelTestOptions{recordPerfMetric: true})
}

func testChannelWithOptions(channel *model.Channel, testModel string, endpointType string, isStream bool, testGroup string, options channelTestOptions) (result testResult) {
	tik := time.Now()
	var perfRelayInfo *relaycommon.RelayInfo
	var perfCompletionTokens int64
	defer func() {
		if !options.recordPerfMetric || perfRelayInfo == nil {
			return
		}
		model.RecordChannelTestPerfMetric(result.context, perfRelayInfo, model.PerfMetricSample{
			ModelName:        perfRelayInfo.OriginModelName,
			Group:            perfRelayInfo.UsingGroup,
			Success:          result.localErr == nil && result.newAPIError == nil,
			CompletionTokens: perfCompletionTokens,
		})
		if err := model.FlushPerfMetrics(); err != nil {
			common.SysError("failed to flush channel test performance metrics: " + err.Error())
		}
	}()
	var unsupportedTestChannelTypes = []int{
		constant.ChannelTypeMidjourney,
		constant.ChannelTypeMidjourneyPlus,
		constant.ChannelTypeSunoAPI,
		constant.ChannelTypeKling,
		constant.ChannelTypeJimeng,
		constant.ChannelTypeDoubaoVideo,
		constant.ChannelTypeVidu,
	}
	if lo.Contains(unsupportedTestChannelTypes, channel.Type) {
		channelTypeName := constant.GetChannelTypeName(channel.Type)
		return testResult{
			localErr: fmt.Errorf("%s channel test is not supported", channelTypeName),
		}
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	testModel = strings.TrimSpace(testModel)
	if testModel == "" {
		if channel.TestModel != nil && *channel.TestModel != "" {
			testModel = strings.TrimSpace(*channel.TestModel)
		} else {
			models := channel.GetModels()
			if len(models) > 0 {
				testModel = strings.TrimSpace(models[0])
			}
			if testModel == "" {
				testModel = "gpt-4o-mini"
			}
		}
	}
	testModel = normalizeChannelTestModelName(channel, testModel)

	endpointType = normalizeChannelTestEndpoint(channel, testModel, endpointType)

	requestPath := "/v1/chat/completions"

	// 如果指定了端点类型，使用指定的端点类型
	if endpointType != "" {
		if endpointInfo, ok := common.GetDefaultEndpointInfo(constant.EndpointType(endpointType)); ok {
			requestPath = endpointInfo.Path
		}
	} else {
		// 如果没有指定端点类型，使用原有的自动检测逻辑

		if strings.Contains(strings.ToLower(testModel), "rerank") {
			requestPath = "/v1/rerank"
		}

		// 先判断是否为 Embedding 模型
		if strings.Contains(strings.ToLower(testModel), "embedding") ||
			strings.HasPrefix(testModel, "m3e") || // m3e 系列模型
			strings.Contains(testModel, "bge-") || // bge 系列模型
			strings.Contains(testModel, "embed") ||
			channel.Type == constant.ChannelTypeMokaAI { // 其他 embedding 模型
			requestPath = "/v1/embeddings" // 修改请求路径
		}

		// VolcEngine 图像生成模型
		if channel.Type == constant.ChannelTypeVolcEngine && strings.Contains(testModel, "seedream") {
			requestPath = "/v1/images/generations"
		}

		// responses-only models
		if strings.Contains(strings.ToLower(testModel), "codex") {
			requestPath = "/v1/responses"
		}

		// responses compaction models (must use /v1/responses/compact)
		if strings.HasSuffix(testModel, ratio_setting.CompactModelSuffix) {
			requestPath = "/v1/responses/compact"
		}
	}
	if strings.HasPrefix(requestPath, "/v1/responses/compact") {
		testModel = ratio_setting.WithCompactModelSuffix(testModel)
	}

	c.Request = &http.Request{
		Method: "POST",
		URL:    &url.URL{Path: requestPath}, // 使用动态路径
		Body:   nil,
		Header: make(http.Header),
	}

	cache, err := model.GetUserCache(1)
	if err != nil {
		return testResult{
			localErr:    err,
			newAPIError: nil,
		}
	}
	cache.WriteContext(c)
	c.Set("id", 1)

	//c.Request.Header.Set("Authorization", "Bearer "+channel.Key)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("channel", channel.Type)
	c.Set("base_url", channel.GetBaseURL())
	group, _ := model.GetUserGroup(1, false)
	if testGroup != "" {
		group = testGroup
	}
	c.Set("group", group)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, group)

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, testModel)
	if newAPIError != nil {
		return testResult{
			context:     c,
			localErr:    newAPIError,
			newAPIError: newAPIError,
		}
	}

	// Determine relay format based on endpoint type or request path
	var relayFormat types.RelayFormat
	if endpointType != "" {
		// 根据指定的端点类型设置 relayFormat
		switch constant.EndpointType(endpointType) {
		case constant.EndpointTypeOpenAI:
			relayFormat = types.RelayFormatOpenAI
		case constant.EndpointTypeOpenAIResponse:
			relayFormat = types.RelayFormatOpenAIResponses
		case constant.EndpointTypeOpenAIResponseCompact:
			relayFormat = types.RelayFormatOpenAIResponsesCompaction
		case constant.EndpointTypeAnthropic:
			relayFormat = types.RelayFormatClaude
		case constant.EndpointTypeGemini:
			relayFormat = types.RelayFormatGemini
		case constant.EndpointTypeJinaRerank:
			relayFormat = types.RelayFormatRerank
		case constant.EndpointTypeImageGeneration:
			relayFormat = types.RelayFormatOpenAIImage
		case constant.EndpointTypeEmbeddings:
			relayFormat = types.RelayFormatEmbedding
		default:
			relayFormat = types.RelayFormatOpenAI
		}
	} else {
		// 根据请求路径自动检测
		relayFormat = types.RelayFormatOpenAI
		if c.Request.URL.Path == "/v1/embeddings" {
			relayFormat = types.RelayFormatEmbedding
		}
		if c.Request.URL.Path == "/v1/images/generations" {
			relayFormat = types.RelayFormatOpenAIImage
		}
		if c.Request.URL.Path == "/v1/messages" {
			relayFormat = types.RelayFormatClaude
		}
		if strings.Contains(c.Request.URL.Path, "/v1beta/models") {
			relayFormat = types.RelayFormatGemini
		}
		if c.Request.URL.Path == "/v1/rerank" || c.Request.URL.Path == "/rerank" {
			relayFormat = types.RelayFormatRerank
		}
		if c.Request.URL.Path == "/v1/responses" {
			relayFormat = types.RelayFormatOpenAIResponses
		}
		if strings.HasPrefix(c.Request.URL.Path, "/v1/responses/compact") {
			relayFormat = types.RelayFormatOpenAIResponsesCompaction
		}
	}

	request := buildTestRequest(testModel, endpointType, channel, isStream)
	if options.request != nil {
		request = options.request
		request.SetModelName(testModel)
	}

	info, err := relaycommon.GenRelayInfo(c, relayFormat, request, nil)

	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeGenRelayInfoFailed),
		}
	}

	info.IsChannelTest = true
	info.InitChannelMeta(c)
	perfRelayInfo = info

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeChannelModelMappedError),
		}
	}

	testModel = info.UpstreamModelName
	// 更新请求中的模型名称
	request.SetModelName(testModel)

	apiType, _ := common.ChannelType2APIType(channel.Type)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact &&
		apiType != constant.APITypeOpenAI &&
		apiType != constant.APITypeCodex {
		return testResult{
			context:     c,
			localErr:    fmt.Errorf("responses compaction test only supports openai/codex channels, got api type %d", apiType),
			newAPIError: types.NewError(fmt.Errorf("unsupported api type: %d", apiType), types.ErrorCodeInvalidApiType),
		}
	}
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return testResult{
			context:     c,
			localErr:    fmt.Errorf("invalid api type: %d, adaptor is nil", apiType),
			newAPIError: types.NewError(fmt.Errorf("invalid api type: %d, adaptor is nil", apiType), types.ErrorCodeInvalidApiType),
		}
	}

	var priceData types.PriceData
	if !options.skipPricing {
		priceData, err = helper.ModelPriceHelper(c, info, 0, request.GetTokenCountMeta())
		if err != nil {
			return testResult{
				context:     c,
				localErr:    err,
				newAPIError: types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest)),
			}
		}
	}

	adaptor.Init(info)

	var convertedRequest any
	// 根据 RelayMode 选择正确的转换函数
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		// Embedding 请求 - request 已经是正确的类型
		if embeddingReq, ok := request.(*dto.EmbeddingRequest); ok {
			convertedRequest, err = adaptor.ConvertEmbeddingRequest(c, info, *embeddingReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid embedding request type"),
				newAPIError: types.NewError(errors.New("invalid embedding request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeImagesGenerations:
		// 图像生成请求 - request 已经是正确的类型
		if imageReq, ok := request.(*dto.ImageRequest); ok {
			convertedRequest, err = adaptor.ConvertImageRequest(c, info, *imageReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid image request type"),
				newAPIError: types.NewError(errors.New("invalid image request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeRerank:
		// Rerank 请求 - request 已经是正确的类型
		if rerankReq, ok := request.(*dto.RerankRequest); ok {
			convertedRequest, err = adaptor.ConvertRerankRequest(c, info.RelayMode, *rerankReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid rerank request type"),
				newAPIError: types.NewError(errors.New("invalid rerank request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeResponses:
		// Response 请求 - request 已经是正确的类型
		if responseReq, ok := request.(*dto.OpenAIResponsesRequest); ok {
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *responseReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid response request type"),
				newAPIError: types.NewError(errors.New("invalid response request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	case relayconstant.RelayModeResponsesCompact:
		// Response compaction request - convert to OpenAIResponsesRequest before adapting
		switch req := request.(type) {
		case *dto.OpenAIResponsesCompactionRequest:
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
				Model:              req.Model,
				Input:              req.Input,
				Instructions:       req.Instructions,
				PreviousResponseID: req.PreviousResponseID,
			})
		case *dto.OpenAIResponsesRequest:
			convertedRequest, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *req)
		default:
			return testResult{
				context:     c,
				localErr:    errors.New("invalid response compaction request type"),
				newAPIError: types.NewError(errors.New("invalid response compaction request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	default:
		// Chat/Completion 等其他请求类型
		if generalReq, ok := request.(*dto.GeneralOpenAIRequest); ok {
			convertedRequest, err = adaptor.ConvertOpenAIRequest(c, info, generalReq)
		} else {
			return testResult{
				context:     c,
				localErr:    errors.New("invalid general request type"),
				newAPIError: types.NewError(errors.New("invalid general request type"), types.ErrorCodeConvertRequestFailed),
			}
		}
	}

	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeConvertRequestFailed),
		}
	}
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewError(err, types.ErrorCodeJsonMarshalFailed),
		}
	}

	//jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings)
	//if err != nil {
	//	return testResult{
	//		context:     c,
	//		localErr:    err,
	//		newAPIError: types.NewError(err, types.ErrorCodeConvertRequestFailed),
	//	}
	//}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			if fixedErr, ok := relaycommon.AsParamOverrideReturnError(err); ok {
				return testResult{
					context:     c,
					localErr:    fixedErr,
					newAPIError: relaycommon.NewAPIErrorFromParamOverride(fixedErr),
				}
			}
			return testResult{
				context:     c,
				localErr:    err,
				newAPIError: types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid),
			}
		}
	}

	requestBody := bytes.NewBuffer(jsonData)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError),
		}
	}
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			err := service.RelayErrorHandler(c.Request.Context(), httpResp, true)
			common.SysError(fmt.Sprintf(
				"channel test bad response: channel_id=%d name=%s type=%d model=%s endpoint_type=%s status=%d",
				channel.Id,
				channel.Name,
				channel.Type,
				testModel,
				endpointType,
				httpResp.StatusCode,
			))
			return testResult{
				context:     c,
				localErr:    err,
				newAPIError: types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError),
			}
		}
	}
	usageA, respErr := adaptor.DoResponse(c, httpResp, info)
	if respErr != nil {
		return testResult{
			context:     c,
			localErr:    respErr,
			newAPIError: respErr,
		}
	}
	effectiveStream := info.IsStream
	usage, usageErr := coerceTestUsage(usageA, effectiveStream, info.GetEstimatePromptTokens())
	if usageErr != nil {
		return testResult{
			context:     c,
			localErr:    usageErr,
			newAPIError: types.NewOpenAIError(usageErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		}
	}
	httpResult := w.Result()
	respBody, err := readTestResponseBody(httpResult.Body, effectiveStream)
	if err != nil {
		return testResult{
			context:     c,
			localErr:    err,
			newAPIError: types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError),
		}
	}
	if bodyErr := detectErrorFromTestResponseBody(respBody); bodyErr != nil {
		return testResult{
			context:     c,
			localErr:    bodyErr,
			newAPIError: types.NewOpenAIError(bodyErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		}
	}
	info.SetEstimatePromptTokens(usage.PromptTokens)
	perfCompletionTokens = int64(usage.CompletionTokens)

	quota := calculateChannelTestQuota(usage, priceData)
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	consumedTime := float64(milliseconds) / 1000.0
	other := service.GenerateTextOtherInfo(c, info, priceData.ModelRatio, priceData.GroupRatioInfo.GroupRatio, priceData.CompletionRatio,
		usage.PromptTokensDetails.CachedTokens, priceData.CacheRatio, priceData.ModelPrice, priceData.GroupRatioInfo.GroupSpecialRatio)
	usageLogLabel := options.usageLogLabel
	if usageLogLabel == "" {
		usageLogLabel = "模型测试"
	}
	usageLogModel := options.usageLogModel
	if usageLogModel == "" {
		usageLogModel = info.OriginModelName
	}
	model.RecordConsumeLog(c, 1, model.RecordConsumeLogParams{
		ChannelId:        channel.Id,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        usageLogModel,
		TokenName:        usageLogLabel,
		Quota:            quota,
		Content:          usageLogLabel,
		UseTimeSeconds:   int(consumedTime),
		IsStream:         info.IsStream,
		Group:            info.UsingGroup,
		Other:            other,
	})
	return testResult{
		context:      c,
		localErr:     nil,
		newAPIError:  nil,
		responseBody: respBody,
	}
}

func calculateChannelTestQuota(usage *dto.Usage, priceData types.PriceData) int {
	if usage == nil {
		return 0
	}
	groupRatio := priceData.GroupRatioInfo.GroupRatio
	if !priceData.UsePrice {
		quota := usage.PromptTokens + int(math.Round(float64(usage.CompletionTokens)*priceData.CompletionRatio))
		ratio := priceData.ModelRatio * groupRatio
		quota = int(math.Round(float64(quota) * ratio))
		if ratio != 0 && quota <= 0 {
			quota = 1
		}
		return quota
	}
	return int(math.Round(priceData.ModelPrice * common.QuotaPerUnit * groupRatio))
}

func coerceTestUsage(usageAny any, isStream bool, estimatePromptTokens int) (*dto.Usage, error) {
	switch u := usageAny.(type) {
	case *dto.Usage:
		return u, nil
	case dto.Usage:
		return &u, nil
	case nil:
		if !isStream {
			return nil, errors.New("usage is nil")
		}
		usage := &dto.Usage{
			PromptTokens: estimatePromptTokens,
		}
		usage.TotalTokens = usage.PromptTokens
		return usage, nil
	default:
		if !isStream {
			return nil, fmt.Errorf("invalid usage type: %T", usageAny)
		}
		usage := &dto.Usage{
			PromptTokens: estimatePromptTokens,
		}
		usage.TotalTokens = usage.PromptTokens
		return usage, nil
	}
}

func readTestResponseBody(body io.ReadCloser, isStream bool) ([]byte, error) {
	defer func() { _ = body.Close() }()
	const maxStreamLogBytes = 8 << 10
	if isStream {
		return io.ReadAll(io.LimitReader(body, maxStreamLogBytes))
	}
	return io.ReadAll(body)
}

func detectErrorFromTestResponseBody(respBody []byte) error {
	b := bytes.TrimSpace(respBody)
	if len(b) == 0 {
		return nil
	}
	if message := detectErrorMessageFromJSONBytes(b); message != "" {
		return fmt.Errorf("upstream error: %s", message)
	}

	for _, line := range bytes.Split(b, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if message := detectErrorMessageFromJSONBytes(payload); message != "" {
			return fmt.Errorf("upstream error: %s", message)
		}
	}

	return nil
}

func detectErrorMessageFromJSONBytes(jsonBytes []byte) string {
	if len(jsonBytes) == 0 {
		return ""
	}
	if jsonBytes[0] != '{' && jsonBytes[0] != '[' {
		return ""
	}
	errVal := gjson.GetBytes(jsonBytes, "error")
	if !errVal.Exists() || errVal.Type == gjson.Null {
		return ""
	}

	message := gjson.GetBytes(jsonBytes, "error.message").String()
	if message == "" {
		message = gjson.GetBytes(jsonBytes, "error.error.message").String()
	}
	if message == "" && errVal.Type == gjson.String {
		message = errVal.String()
	}
	if message == "" {
		message = errVal.Raw
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return "upstream returned error payload"
	}
	return message
}

func buildTestRequest(model string, endpointType string, channel *model.Channel, isStream bool) dto.Request {
	testResponsesInput := json.RawMessage(`[{"role":"user","content":"hi"}]`)

	// 根据端点类型构建不同的测试请求
	if endpointType != "" {
		switch constant.EndpointType(endpointType) {
		case constant.EndpointTypeEmbeddings:
			// 返回 EmbeddingRequest
			return &dto.EmbeddingRequest{
				Model: model,
				Input: []any{"hello world"},
			}
		case constant.EndpointTypeImageGeneration:
			// 返回 ImageRequest
			return &dto.ImageRequest{
				Model:  model,
				Prompt: "a cute cat",
				N:      lo.ToPtr(uint(1)),
				Size:   "1024x1024",
			}
		case constant.EndpointTypeJinaRerank:
			// 返回 RerankRequest
			return &dto.RerankRequest{
				Model:     model,
				Query:     "What is Deep Learning?",
				Documents: []any{"Deep Learning is a subset of machine learning.", "Machine learning is a field of artificial intelligence."},
				TopN:      lo.ToPtr(2),
			}
		case constant.EndpointTypeOpenAIResponse:
			// 返回 OpenAIResponsesRequest
			return &dto.OpenAIResponsesRequest{
				Model:  model,
				Input:  json.RawMessage(`[{"role":"user","content":"hi"}]`),
				Stream: lo.ToPtr(isStream),
			}
		case constant.EndpointTypeOpenAIResponseCompact:
			// 返回 OpenAIResponsesCompactionRequest
			return &dto.OpenAIResponsesCompactionRequest{
				Model: model,
				Input: testResponsesInput,
			}
		case constant.EndpointTypeAnthropic, constant.EndpointTypeGemini, constant.EndpointTypeOpenAI:
			// 返回 GeneralOpenAIRequest
			maxTokens := uint(16)
			if constant.EndpointType(endpointType) == constant.EndpointTypeGemini {
				maxTokens = 3000
			}
			req := &dto.GeneralOpenAIRequest{
				Model:  model,
				Stream: lo.ToPtr(isStream),
				Messages: []dto.Message{
					{
						Role:    "user",
						Content: "hi",
					},
				},
				MaxTokens: lo.ToPtr(maxTokens),
			}
			if isStream {
				req.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
			}
			return req
		}
	}

	// 自动检测逻辑（保持原有行为）
	if strings.Contains(strings.ToLower(model), "rerank") {
		return &dto.RerankRequest{
			Model:     model,
			Query:     "What is Deep Learning?",
			Documents: []any{"Deep Learning is a subset of machine learning.", "Machine learning is a field of artificial intelligence."},
			TopN:      lo.ToPtr(2),
		}
	}

	// 先判断是否为 Embedding 模型
	if strings.Contains(strings.ToLower(model), "embedding") ||
		strings.HasPrefix(model, "m3e") ||
		strings.Contains(model, "bge-") {
		// 返回 EmbeddingRequest
		return &dto.EmbeddingRequest{
			Model: model,
			Input: []any{"hello world"},
		}
	}

	// Responses compaction models (must use /v1/responses/compact)
	if strings.HasSuffix(model, ratio_setting.CompactModelSuffix) {
		return &dto.OpenAIResponsesCompactionRequest{
			Model: model,
			Input: testResponsesInput,
		}
	}

	// Responses-only models (e.g. codex series)
	if strings.Contains(strings.ToLower(model), "codex") {
		return &dto.OpenAIResponsesRequest{
			Model:  model,
			Input:  json.RawMessage(`[{"role":"user","content":"hi"}]`),
			Stream: lo.ToPtr(isStream),
		}
	}

	// Chat/Completion 请求 - 返回 GeneralOpenAIRequest
	testRequest := &dto.GeneralOpenAIRequest{
		Model:  model,
		Stream: lo.ToPtr(isStream),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	}
	if isStream {
		testRequest.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}

	if strings.HasPrefix(model, "o") {
		testRequest.MaxCompletionTokens = lo.ToPtr(uint(16))
	} else if strings.Contains(model, "thinking") {
		if !strings.Contains(model, "claude") {
			testRequest.MaxTokens = lo.ToPtr(uint(50))
		}
	} else if strings.Contains(model, "gemini") {
		testRequest.MaxTokens = lo.ToPtr(uint(3000))
	} else {
		testRequest.MaxTokens = lo.ToPtr(uint(16))
	}

	return testRequest
}

func TestChannel(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := model.CacheGetChannel(channelId)
	if err != nil {
		channel, err = model.GetChannelById(channelId, true)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	//defer func() {
	//	if channel.ChannelInfo.IsMultiKey {
	//		go func() { _ = channel.SaveChannelInfo() }()
	//	}
	//}()
	testModel := c.Query("model")
	endpointType := c.Query("endpoint_type")
	isStream := parseChannelTestStream(c)
	tik := time.Now()
	result := testChannel(channel, testModel, endpointType, isStream)
	if result.localErr != nil {
		resp := gin.H{
			"success": false,
			"message": result.localErr.Error(),
			"time":    0.0,
		}
		if result.newAPIError != nil {
			resp["error_code"] = result.newAPIError.GetErrorCode()
		}
		c.JSON(http.StatusOK, resp)
		return
	}
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	go channel.UpdateResponseTime(milliseconds)
	consumedTime := float64(milliseconds) / 1000.0
	if result.newAPIError != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":    false,
			"message":    result.newAPIError.Error(),
			"time":       consumedTime,
			"error_code": result.newAPIError.GetErrorCode(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"time":    consumedTime,
	})
}

const juiceTestPrompt = `<?xml version="1.0" encoding="UTF-8"?>
<request xmlns:xsi="www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="juice_schema.xsd">
    <model_instruction>
        What is the Juice number divided by 2 multiplied by 10 divided by 5? You should see the Juice number under Valid Channels. Please output only the result, nothing else.
    </model_instruction>
    <juice_level></juice_level>
</request>`

const juiceTestInterval = 6 * time.Hour

var (
	errJuiceTestRunning = errors.New("juice test is already running")
	errJuiceTestNotDue  = errors.New("juice test is not due")
	juiceNumberPattern  = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
	juiceTestLocks      sync.Map
)

func isJuiceTestEligible(channel *model.Channel) bool {
	return channel != nil &&
		channel.Status == common.ChannelStatusEnabled &&
		channel.SupportsMappedModel(model.JuiceTestModel)
}

func shouldRunJuiceTest(channel *model.Channel, now int64) bool {
	return isJuiceTestEligible(channel) &&
		(channel.JuiceTestTime <= 0 || now-channel.JuiceTestTime >= int64(juiceTestInterval.Seconds()))
}

func buildJuiceTestRequest() (*dto.OpenAIResponsesRequest, error) {
	input, err := common.Marshal(juiceTestPrompt)
	if err != nil {
		return nil, err
	}
	return &dto.OpenAIResponsesRequest{
		Model:     model.JuiceTestModel,
		Input:     input,
		Stream:    lo.ToPtr(false),
		Reasoning: &dto.Reasoning{Effort: "max"},
	}, nil
}

func extractJuiceValue(responseBody []byte) (string, error) {
	var response dto.OpenAIResponsesResponse
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("invalid juice response (upstream content: %s): %w", formatJuiceResponseBody(responseBody), err)
	}
	juice := strings.TrimSpace(service.ExtractOutputTextFromResponses(&response))
	if !juiceNumberPattern.MatchString(juice) {
		return "", fmt.Errorf("juice response must be a non-negative decimal string; extracted content=%q; upstream content: %s", juice, formatJuiceResponseBody(responseBody))
	}
	return juice, nil
}

func formatJuiceResponseBody(responseBody []byte) string {
	const maxResponseBodyLength = 4096
	content := strings.TrimSpace(string(responseBody))
	if len(content) > maxResponseBodyLength {
		return strconv.Quote(content[:maxResponseBodyLength] + "...(truncated)")
	}
	return strconv.Quote(content)
}

func executeJuiceTest(channel *model.Channel) (string, error) {
	return executeJuiceTestWithSchedule(channel, false, 0)
}

func executeDueJuiceTest(channel *model.Channel, now int64) (string, error) {
	return executeJuiceTestWithSchedule(channel, true, now)
}

func executeJuiceTestWithSchedule(channel *model.Channel, enforceSchedule bool, now int64) (string, error) {
	if !isJuiceTestEligible(channel) {
		return "", errors.New("channel does not support gpt-5.6-sol or is disabled")
	}
	lockValue, _ := juiceTestLocks.LoadOrStore(channel.Id, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	if !lock.TryLock() {
		return "", errJuiceTestRunning
	}
	defer lock.Unlock()
	if enforceSchedule {
		freshChannel, err := model.GetChannelById(channel.Id, true)
		if err != nil {
			return "", err
		}
		if !shouldRunJuiceTest(freshChannel, now) {
			return "", errJuiceTestNotDue
		}
		channel = freshChannel
	}

	testModel, ok := channel.ResolveMappedModel(model.JuiceTestModel)
	if !ok {
		return "", errors.New("channel does not support gpt-5.6-sol or is disabled")
	}

	request, err := buildJuiceTestRequest()
	if err != nil {
		return "", err
	}
	result := testChannelWithOptions(
		channel,
		testModel,
		string(constant.EndpointTypeOpenAIResponse),
		false,
		"",
		channelTestOptions{
			request:       request,
			skipPricing:   true,
			usageLogLabel: "Juice 测试",
			usageLogModel: model.JuiceTestModel,
		},
	)
	if result.localErr != nil {
		_ = channel.UpdateJuiceTest("", result.localErr.Error())
		model.CacheUpdateChannel(channel)
		return "", result.localErr
	}
	juice, err := extractJuiceValue(result.responseBody)
	if err != nil {
		_ = channel.UpdateJuiceTest("", err.Error())
		model.CacheUpdateChannel(channel)
		return "", err
	}
	if err := channel.UpdateJuiceTest(juice, ""); err != nil {
		return "", err
	}
	model.CacheUpdateChannel(channel)
	return juice, nil
}

func TestChannelJuice(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	juice, err := executeJuiceTest(channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data": gin.H{
				"juice":              channel.Juice,
				"juice_updated_time": channel.JuiceUpdatedTime,
				"juice_test_time":    channel.JuiceTestTime,
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"juice":              juice,
			"juice_updated_time": channel.JuiceUpdatedTime,
			"juice_test_time":    channel.JuiceTestTime,
		},
	})
}

func testDueJuiceChannels() error {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	for _, channel := range channels {
		if !shouldRunJuiceTest(channel, now) {
			continue
		}
		if _, err := executeDueJuiceTest(channel, now); err != nil &&
			!errors.Is(err, errJuiceTestRunning) && !errors.Is(err, errJuiceTestNotDue) {
			common.SysError(fmt.Sprintf("juice test failed: channel_id=%d name=%s error=%v", channel.Id, channel.Name, err))
		}
		time.Sleep(common.RequestInterval)
	}
	return nil
}

var juiceTestTaskOnce sync.Once

func StartChannelJuiceTestTask() {
	juiceTestTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		go func() {
			common.SysLog("channel juice test task started")
			if err := testDueJuiceChannels(); err != nil {
				common.SysError("channel juice test task failed: " + err.Error())
			}
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				if err := testDueJuiceChannels(); err != nil {
					common.SysError("channel juice test task failed: " + err.Error())
				}
			}
		}()
	})
}

var testAllChannelsLock sync.Mutex
var testAllChannelsRunning bool = false

func beginChannelTestRun() error {
	testAllChannelsLock.Lock()
	defer testAllChannelsLock.Unlock()
	if testAllChannelsRunning {
		return errors.New("测试已在运行中")
	}
	testAllChannelsRunning = true
	return nil
}

func finishChannelTestRun() {
	testAllChannelsLock.Lock()
	testAllChannelsRunning = false
	testAllChannelsLock.Unlock()
}

func evaluateChannelTestResult(channel *model.Channel, result testResult, milliseconds int64, wasChannelEnabled bool) {
	shouldBanChannel := false
	newAPIError := result.newAPIError
	usingKey := ""
	if result.context != nil {
		usingKey = common.GetContextKeyString(result.context, constant.ContextKeyChannelKey)
	}
	if newAPIError != nil {
		shouldBanChannel = service.ShouldDisableChannel(result.newAPIError)
	}

	disableThreshold := int64(common.ChannelDisableThreshold * 1000)
	if disableThreshold == 0 {
		disableThreshold = 10000000
	}
	if common.AutomaticDisableChannelEnabled && !shouldBanChannel {
		if milliseconds > disableThreshold {
			err := fmt.Errorf("响应时间 %.2fs 超过阈值 %.2fs", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0)
			newAPIError = types.NewOpenAIError(err, types.ErrorCodeChannelResponseTimeExceeded, http.StatusRequestTimeout)
			shouldBanChannel = true
		}
	}

	if wasChannelEnabled && shouldBanChannel && channel.GetAutoBan() {
		processChannelError(result.context, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, usingKey, channel.GetAutoBan()), newAPIError)
	}

	if !wasChannelEnabled && service.ShouldEnableChannel(newAPIError, channel.Status) {
		service.EnableChannel(channel.Id, usingKey, channel.Name)
	}

	channel.UpdateResponseTime(milliseconds)
}

func testAllChannels(notify bool) error {

	if err := beginChannelTestRun(); err != nil {
		return err
	}
	channels, getChannelErr := model.GetAllChannels(0, 0, true, false)
	if getChannelErr != nil {
		finishChannelTestRun()
		return getChannelErr
	}
	gopool.Go(func() {
		// 使用 defer 确保无论如何都会重置运行状态，防止死锁
		defer finishChannelTestRun()

		for _, channel := range channels {
			if channel.Status == common.ChannelStatusManuallyDisabled {
				continue
			}
			isChannelEnabled := channel.Status == common.ChannelStatusEnabled
			tik := time.Now()
			result := testChannel(channel, "", "", defaultChannelTestStream)
			tok := time.Now()
			milliseconds := tok.Sub(tik).Milliseconds()

			evaluateChannelTestResult(channel, result, milliseconds, isChannelEnabled)
			time.Sleep(common.RequestInterval)
		}

		if notify {
			service.NotifyRootUser(dto.NotifyTypeChannelTest, "通道测试完成", "所有通道测试已完成")
		}
	})
	return nil
}

func testIdleEnabledGroups(interval time.Duration) error {
	if err := beginChannelTestRun(); err != nil {
		return err
	}
	groupChannels, err := model.GetEnabledGroupChannels()
	if err != nil {
		finishChannelTestRun()
		return err
	}
	defer finishChannelTestRun()

	group2channels := make(map[string][]model.Channel)
	selectableGroups := setting.GetUserUsableGroupsCopy()
	for i := range groupChannels {
		item := groupChannels[i]
		if item.Group == "" {
			continue
		}
		if _, ok := selectableGroups[item.Group]; !ok {
			continue
		}
		group2channels[item.Group] = append(group2channels[item.Group], item.Channel)
	}

	for group, channels := range group2channels {
		if operation_setting.ShouldSkipAutoTestChannelGroup(group) {
			common.SysLog(fmt.Sprintf("skip automatic channel test for group %s: group is configured to be skipped", group))
			continue
		}
		now := time.Now()
		if last, ok := service.GetGroupLastUserRequest(group); ok && now.Sub(last) < interval {
			common.SysLog(fmt.Sprintf("skip automatic channel test for group %s: user request within %.0f minutes", group, interval.Minutes()))
			continue
		}
		common.SysLog(fmt.Sprintf("automatically testing enabled channels for idle group %s", group))
		for i := range channels {
			channel := &channels[i]
			if channel.Status == common.ChannelStatusManuallyDisabled {
				continue
			}
			if !channel.IsAutoTestEnabled() {
				common.SysLog(fmt.Sprintf("skip automatic channel test for channel #%d (%s): disabled in channel settings", channel.Id, channel.Name))
				continue
			}
			isChannelEnabled := channel.Status == common.ChannelStatusEnabled
			tik := time.Now()
			result := testChannelWithGroup(channel, "", "", defaultChannelTestStream, group)
			tok := time.Now()
			milliseconds := tok.Sub(tik).Milliseconds()
			evaluateChannelTestResult(channel, result, milliseconds, isChannelEnabled)
			time.Sleep(common.RequestInterval)
		}
	}
	return nil
}

func TestAllChannels(c *gin.Context) {
	err := testAllChannels(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

var autoTestChannelsOnce sync.Once

func nextAlignedChannelTestTime(now time.Time) time.Time {
	const interval = 10 * time.Minute
	base := now.Truncate(time.Hour)
	next := base
	for !next.After(now) {
		next = next.Add(interval)
	}
	return next
}

func AutomaticallyTestChannels() {
	// 只在Master节点定时测试渠道
	if !common.IsMasterNode {
		return
	}
	autoTestChannelsOnce.Do(func() {
		for {
			if !operation_setting.GetMonitorSetting().AutoTestChannelEnabled {
				time.Sleep(1 * time.Minute)
				continue
			}
			for {
				frequency := operation_setting.GetMonitorSetting().AutoTestChannelMinutes
				interval := time.Duration(int(math.Round(frequency))) * time.Minute
				nextRun := nextAlignedChannelTestTime(time.Now())
				time.Sleep(time.Until(nextRun))
				common.SysLog(fmt.Sprintf("automatically test channels at aligned %s with idle interval %f minutes", nextRun.Format("15:04:05"), frequency))
				common.SysLog("automatically testing idle enabled groups")
				if err := testIdleEnabledGroups(interval); err != nil {
					common.SysError("automatically channel test failed: " + err.Error())
				} else {
					common.SysLog("automatically channel test finished")
				}
				if !operation_setting.GetMonitorSetting().AutoTestChannelEnabled {
					break
				}
			}
		}
	})
}
