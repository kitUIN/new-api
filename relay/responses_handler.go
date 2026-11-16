package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		default:
			return types.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:              req.Model,
			Input:              req.Input,
			Instructions:       req.Instructions,
			PreviousResponseID: req.PreviousResponseID,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	stageStart := time.Now()
	request, err := common.DeepCopy(responsesReq)
	info.RecordPreparationDuration("responses_copy", time.Since(stageStart))
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	// Retain background only for the WebSocket transport's validation. The
	// existing HTTP conversion does not support background-response polling.
	if !channel.UseResponsesWebSocket(info) {
		request.Background = nil
	}
	filteredResponsesToolTypes := getFilteredResponsesToolTypes(info.ChannelOtherSettings)
	if len(filteredResponsesToolTypes) > 0 {
		if err = request.RemoveTools(filteredResponsesToolTypes...); err != nil {
			return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	var requestBody io.Reader
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		rawBody, err := storage.Bytes()
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		jsonData := rawBody
		if len(filteredResponsesToolTypes) > 0 {
			jsonData, err = removeResponsesToolsFromBody(rawBody, filteredResponsesToolTypes...)
			if err != nil {
				return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
			}
		}
		requestBody = bytes.NewBuffer(jsonData)
	} else {
		stageStart = time.Now()
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		info.RecordPreparationDuration("responses_convert", time.Since(stageStart))
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		stageStart = time.Now()
		jsonData, err := common.Marshal(convertedRequest)
		info.RecordPreparationDuration("responses_marshal", time.Since(stageStart))
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI Responses API
		stageStart = time.Now()
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		info.RecordPreparationDuration("responses_filter", time.Since(stageStart))
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			stageStart = time.Now()
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			info.RecordPreparationDuration("param_override", time.Since(stageStart))
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}

		if len(filteredResponsesToolTypes) > 0 {
			stageStart = time.Now()
			jsonData, err = removeResponsesToolsFromBody(jsonData, filteredResponsesToolTypes...)
			info.RecordPreparationDuration("responses_filter", time.Since(stageStart))
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
		}

		if common.DebugEnabled {
			println("requestBody: ", string(jsonData))
		}
		requestBody = bytes.NewBuffer(jsonData)
	}

	stageStart = time.Now()
	reqBodyForLog := requestBodySnapshot(c, requestBody)
	info.RecordPreparationDuration("request_snapshot", time.Since(stageStart))

	var httpResp *http.Response
	recordDetail := buildRecordDetailFunc(c, info, reqBodyForLog, &httpResp)
	defer recordDetail(false)

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		recordDetail(true)
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)

		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			recordDetail(true)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		recordDetail(true)
		return newAPIError
	}

	usageDto := usage.(*dto.Usage)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData

		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
		}
		service.PostTextConsumeQuota(c, info, usageDto, nil)

		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
		service.PostAudioConsumeQuota(c, info, usageDto, "")
	} else {
		service.PostTextConsumeQuota(c, info, usageDto, nil)
	}
	return nil
}

func getFilteredResponsesToolTypes(settings dto.ChannelOtherSettings) []string {
	filteredToolTypes := make([]string, 0, 2)
	if !settings.DisableResponsesImageGenerationFilter {
		filteredToolTypes = append(filteredToolTypes, dto.OpenAIResponsesToolTypeImageGeneration)
	}
	if settings.DisableResponsesWebSearchTool {
		filteredToolTypes = append(filteredToolTypes, dto.OpenAIResponsesToolTypeWebSearch)
	}
	return filteredToolTypes
}

func removeResponsesImageGenerationToolsFromBody(jsonData []byte) ([]byte, error) {
	return removeResponsesToolsFromBody(jsonData, dto.OpenAIResponsesToolTypeImageGeneration)
}

func removeResponsesWebSearchToolsFromBody(jsonData []byte) ([]byte, error) {
	return removeResponsesToolsFromBody(jsonData, dto.OpenAIResponsesToolTypeWebSearch)
}

func removeResponsesToolsFromBody(jsonData []byte, filteredToolTypes ...string) ([]byte, error) {
	// Tools are usually unchanged. Inspect only that value before allocating a
	// RawMessage copy of every field (especially the large input field).
	if gjson.ValidBytes(jsonData) {
		root := gjson.ParseBytes(jsonData)
		if root.IsObject() {
			var tools gjson.Result
			root.ForEach(func(key, value gjson.Result) bool {
				if key.Str == "tools" {
					tools = value
				}
				return true
			})
			_, removed, err := dto.FilterOpenAIResponsesTools([]byte(tools.Raw), filteredToolTypes...)
			if err != nil {
				return nil, err
			}
			if !removed {
				return jsonData, nil
			}
		}
	}
	var request map[string]json.RawMessage
	if err := common.Unmarshal(jsonData, &request); err != nil {
		return nil, err
	}

	tools, ok := request["tools"]
	if !ok {
		return jsonData, nil
	}

	filteredTools, removed, err := dto.FilterOpenAIResponsesTools(tools, filteredToolTypes...)
	if err != nil {
		return nil, err
	}
	if !removed {
		return jsonData, nil
	}
	if len(filteredTools) == 0 {
		delete(request, "tools")
	} else {
		request["tools"] = filteredTools
	}
	return common.Marshal(request)
}
