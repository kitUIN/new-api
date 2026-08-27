package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	responsesWebSocketStateIdle       = "idle"
	responsesWebSocketStateWarming    = "warming"
	responsesWebSocketStateGenerating = "generating"
	responsesWebSocketStateClosing    = "closing"

	responsesWebSocketMaxMessageBytes = 16 << 20
	responsesWebSocketPongTimeout     = 75 * time.Second
	responsesWebSocketPingInterval    = 25 * time.Second
	responsesWebSocketMaxLifetime     = time.Hour
)

type ResponsesWebSocketStats struct {
	ActiveConnections int64
	ConnectionMillis  int64
	UpstreamDialFails int64
	Rounds            int64
	SuccessfulRounds  int64
}

var responsesWebSocketStats struct {
	activeConnections atomic.Int64
	connectionMillis  atomic.Int64
	upstreamDialFails atomic.Int64
	rounds            atomic.Int64
	successfulRounds  atomic.Int64
}

func GetResponsesWebSocketStats() ResponsesWebSocketStats {
	return ResponsesWebSocketStats{
		ActiveConnections: responsesWebSocketStats.activeConnections.Load(),
		ConnectionMillis:  responsesWebSocketStats.connectionMillis.Load(),
		UpstreamDialFails: responsesWebSocketStats.upstreamDialFails.Load(),
		Rounds:            responsesWebSocketStats.rounds.Load(),
		SuccessfulRounds:  responsesWebSocketStats.successfulRounds.Load(),
	}
}

func WriteResponsesWebSocketError(c *gin.Context, ws *websocket.Conn, status int, code string, err error) error {
	if ws == nil {
		return errors.New("websocket connection is nil")
	}
	message := "request failed"
	if err != nil {
		message = err.Error()
	}
	if c != nil {
		requestID := c.GetString(common.RequestIdKey)
		if requestID == "" || !strings.Contains(message, requestID) {
			message = common.MessageWithRequestId(message, requestID)
		}
	}
	message = types.LimitErrorMessageForResponse(common.MaskSensitiveInfo(message))
	if strings.TrimSpace(code) == "" {
		code = string(types.ErrorCodeInvalidRequest)
	}
	payload, marshalErr := common.Marshal(dto.NewOpenAIResponsesWebSocketErrorEvent(status, code, message))
	if marshalErr != nil {
		return marshalErr
	}
	_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ws.WriteMessage(websocket.TextMessage, payload)
}

func DialResponsesWebSocketUpstream(c *gin.Context, info *relaycommon.RelayInfo) (*websocket.Conn, *types.NewAPIError) {
	if info == nil {
		return nil, types.NewError(errors.New("relay info is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	info.InitChannelMeta(c)
	if !info.ChannelOtherSettings.IsResponsesWebSocketEnabled() {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("channel %d explicitly disables Responses WebSocket support", info.ChannelId),
			types.ErrorCodeChannelResponsesWebSocketUnsupported,
			http.StatusBadGateway,
		)
	}
	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	target, err := channel.DoWssRequest(adaptor, c, info, nil)
	if err != nil {
		responsesWebSocketStats.upstreamDialFails.Add(1)
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeChannelResponsesWebSocketDialFailed, http.StatusBadGateway)
	}
	info.TargetWs = target
	return target, nil
}

type responsesWebSocketFrame struct {
	messageType int
	data        []byte
	err         error
}

type responsesWebSocketSession struct {
	c               *gin.Context
	connectionInfo  *relaycommon.RelayInfo
	client          *websocket.Conn
	upstream        *websocket.Conn
	state           string
	round           *relaycommon.RelayInfo
	roundNumber     int64
	connectionModel string
}

func RunResponsesWebSocket(c *gin.Context, info *relaycommon.RelayInfo, firstFrame []byte) error {
	if info == nil || info.ClientWs == nil || info.TargetWs == nil {
		return errors.New("responses websocket connections are not initialized")
	}
	session := &responsesWebSocketSession{
		c:               c,
		connectionInfo:  info,
		client:          info.ClientWs,
		upstream:        info.TargetWs,
		state:           responsesWebSocketStateIdle,
		connectionModel: info.OriginModelName,
	}
	startedAt := time.Now()
	responsesWebSocketStats.activeConnections.Add(1)
	defer func() {
		responsesWebSocketStats.activeConnections.Add(-1)
		responsesWebSocketStats.connectionMillis.Add(time.Since(startedAt).Milliseconds())
		session.refundRound()
		session.state = responsesWebSocketStateClosing
		info.ResponsesWebSocketState = responsesWebSocketStateClosing
	}()
	done := make(chan struct{})
	var readers sync.WaitGroup
	defer func() {
		close(done)
		_ = session.client.Close()
		_ = session.upstream.Close()
		readers.Wait()
	}()

	session.configureConnection(session.client)
	session.configureConnection(session.upstream)
	if apiErr := session.handleClientFrame(websocket.TextMessage, firstFrame); apiErr != nil {
		_ = WriteResponsesWebSocketError(c, session.client, apiErr.StatusCode, string(apiErr.GetErrorCode()), apiErr)
		return apiErr
	}

	clientFrames := make(chan responsesWebSocketFrame, 1)
	upstreamFrames := make(chan responsesWebSocketFrame, 1)
	readers.Add(2)
	go func() {
		defer readers.Done()
		readResponsesWebSocketFrames(session.client, clientFrames, done)
	}()
	go func() {
		defer readers.Done()
		readResponsesWebSocketFrames(session.upstream, upstreamFrames, done)
	}()

	pingTicker := time.NewTicker(responsesWebSocketPingInterval)
	lifetime := time.NewTimer(responsesWebSocketMaxLifetime)
	defer pingTicker.Stop()
	defer lifetime.Stop()

	for {
		select {
		case frame := <-clientFrames:
			if frame.err != nil {
				_ = writeWebSocketClose(session.upstream, websocket.CloseGoingAway, "client disconnected")
				return frame.err
			}
			if apiErr := session.handleClientFrame(frame.messageType, frame.data); apiErr != nil {
				_ = WriteResponsesWebSocketError(c, session.client, apiErr.StatusCode, string(apiErr.GetErrorCode()), apiErr)
			}
		case frame := <-upstreamFrames:
			if frame.err != nil {
				session.refundRound()
				if !websocket.IsCloseError(frame.err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					_ = WriteResponsesWebSocketError(c, session.client, http.StatusBadGateway, string(types.ErrorCodeBadResponse), errors.New("upstream websocket disconnected"))
				}
				_ = writeWebSocketClose(session.client, websocket.CloseGoingAway, "upstream disconnected")
				return frame.err
			}
			if err := session.handleUpstreamFrame(frame.messageType, frame.data); err != nil {
				session.refundRound()
				_ = WriteResponsesWebSocketError(c, session.client, http.StatusBadGateway, string(types.ErrorCodeBadResponse), err)
				return err
			}
		case <-pingTicker.C:
			deadline := time.Now().Add(10 * time.Second)
			if err := session.client.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				return err
			}
			if err := session.upstream.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				return err
			}
		case <-lifetime.C:
			_ = writeWebSocketClose(session.client, websocket.CloseNormalClosure, "connection lifetime reached")
			_ = writeWebSocketClose(session.upstream, websocket.CloseNormalClosure, "connection lifetime reached")
			return nil
		}
	}
}

func (s *responsesWebSocketSession) configureConnection(conn *websocket.Conn) {
	conn.SetReadLimit(responsesWebSocketMaxMessageBytes)
	_ = conn.SetReadDeadline(time.Now().Add(responsesWebSocketPongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(responsesWebSocketPongTimeout))
	})
}

func readResponsesWebSocketFrames(conn *websocket.Conn, output chan<- responsesWebSocketFrame, done <-chan struct{}) {
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			select {
			case output <- responsesWebSocketFrame{err: err}:
			case <-done:
			}
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(responsesWebSocketPongTimeout))
		select {
		case output <- responsesWebSocketFrame{messageType: messageType, data: data}:
		case <-done:
			return
		}
	}
}

func writeWebSocketClose(conn *websocket.Conn, code int, message string) error {
	if conn == nil {
		return nil
	}
	return conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, message), time.Now().Add(5*time.Second))
}

func (s *responsesWebSocketSession) handleClientFrame(messageType int, data []byte) *types.NewAPIError {
	if messageType != websocket.TextMessage {
		return types.NewErrorWithStatusCode(errors.New("only text WebSocket messages are supported"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if s.state != responsesWebSocketStateIdle {
		return types.NewErrorWithStatusCode(errors.New("a response is already in progress on this connection"), types.ErrorCodeInvalidRequest, http.StatusConflict, types.ErrOptionWithSkipRetry())
	}
	event, err := dto.ParseOpenAIResponsesWebSocketEvent(data)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if event.Model == "" {
		event.Model = s.connectionModel
	}
	if event.Model != s.connectionModel {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("model %q does not match connection model %q", event.Model, s.connectionModel),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	hasBackground, err := dto.HasOpenAIResponsesWebSocketBackground(data)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if hasBackground {
		return types.NewErrorWithStatusCode(errors.New("background is not supported in Responses WebSocket mode"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	round, payload, apiErr := prepareResponsesWebSocketRound(s.c, s.connectionInfo, event, data, s.roundNumber+1)
	if apiErr != nil {
		return apiErr
	}
	generate := event.Generate == nil || *event.Generate
	if apiErr = preConsumeResponsesWebSocketRound(s.c, round, generate); apiErr != nil {
		return apiErr
	}
	s.roundNumber++
	s.round = round
	responsesWebSocketStats.rounds.Add(1)
	if generate {
		s.state = responsesWebSocketStateGenerating
	} else {
		s.state = responsesWebSocketStateWarming
	}
	s.connectionInfo.ResponsesWebSocketState = s.state
	_ = s.upstream.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err = s.upstream.WriteMessage(websocket.TextMessage, payload); err != nil {
		s.refundRound()
		return types.NewErrorWithStatusCode(err, types.ErrorCodeChannelResponsesWebSocketDialFailed, http.StatusBadGateway)
	}
	return nil
}

func (s *responsesWebSocketSession) handleUpstreamFrame(messageType int, data []byte) error {
	if messageType == websocket.BinaryMessage {
		return errors.New("upstream Responses WebSocket returned an unsupported binary message")
	}
	if messageType != websocket.TextMessage {
		return nil
	}
	if s.round != nil {
		s.round.SetFirstResponseTime()
		s.round.SendResponseCount++
	}
	_ = s.client.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := s.client.WriteMessage(messageType, data); err != nil {
		return err
	}
	var event dto.ResponsesStreamResponse
	if err := common.Unmarshal(data, &event); err != nil {
		logger.LogError(s.c, "failed to parse Responses WebSocket upstream event: "+err.Error())
		return nil
	}
	s.handleUpstreamEvent(event)
	return nil
}

func (s *responsesWebSocketSession) handleUpstreamEvent(event dto.ResponsesStreamResponse) {
	s.observeBuiltInTool(event)
	switch event.Type {
	case "response.completed", "response.incomplete":
		s.completeRound(event.Response)
	case "response.failed", "error":
		s.refundRound()
	}
}

func (s *responsesWebSocketSession) observeBuiltInTool(event dto.ResponsesStreamResponse) {
	if s.round == nil || event.Type != dto.ResponsesOutputTypeItemDone || event.Item == nil || s.round.ResponsesUsageInfo == nil {
		return
	}
	if event.Item.Type != dto.BuildInCallWebSearchCall {
		return
	}
	if tool := s.round.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; tool != nil {
		tool.CallCount++
	}
}

func (s *responsesWebSocketSession) completeRound(response *dto.OpenAIResponsesResponse) {
	if s.round == nil {
		s.state = responsesWebSocketStateIdle
		s.connectionInfo.ResponsesWebSocketState = s.state
		return
	}
	usage := &dto.Usage{}
	if response != nil && response.Usage != nil {
		usage.PromptTokens = response.Usage.InputTokens
		usage.CompletionTokens = response.Usage.OutputTokens
		usage.TotalTokens = response.Usage.TotalTokens
		if response.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = response.Usage.InputTokensDetails.CachedTokens
		}
		if response.HasImageGenerationCall() {
			s.c.Set("image_generation_call", true)
			s.c.Set("image_generation_call_quality", response.GetQuality())
			s.c.Set("image_generation_call_size", response.GetSize())
		}
	}
	if shouldRecordResponsesWebSocketConsumption(s.state, usage) {
		if usage.TotalTokens > 0 && !s.round.PriceData.FreeModel && s.round.Billing == nil {
			if apiErr := service.PreConsumeBilling(s.c, 0, s.round); apiErr != nil {
				logger.LogError(s.c, "failed to initialize deferred Responses WebSocket billing: "+apiErr.Error())
			}
		}
		baseRequestID := s.c.GetString(common.RequestIdKey)
		s.c.Set(common.RequestIdKey, s.round.RequestId)
		service.PostTextConsumeQuota(s.c, s.round, usage, []string{"Responses WebSocket 回合"})
		s.c.Set(common.RequestIdKey, baseRequestID)
	}
	responsesWebSocketStats.successfulRounds.Add(1)
	ttftMs := int64(0)
	hasTTFT := s.round.FirstResponseTime.After(s.round.StartTime)
	if hasTTFT {
		ttftMs = s.round.FirstResponseTime.Sub(s.round.StartTime).Milliseconds()
	}
	service.RecordRuleAutoGroupResult(s.c, true, false, ttftMs, hasTTFT)
	service.RecordSessionGroupFailoverResult(s.c, true)
	s.round = nil
	s.state = responsesWebSocketStateIdle
	s.connectionInfo.ResponsesWebSocketState = s.state
}

func shouldRecordResponsesWebSocketConsumption(state string, usage *dto.Usage) bool {
	return state != responsesWebSocketStateWarming || (usage != nil && usage.TotalTokens > 0)
}

func (s *responsesWebSocketSession) refundRound() {
	if s.round != nil && s.round.Billing != nil {
		s.round.Billing.Refund(s.c)
	}
	s.round = nil
	s.state = responsesWebSocketStateIdle
	s.connectionInfo.ResponsesWebSocketState = s.state
}

func prepareResponsesWebSocketRound(c *gin.Context, connectionInfo *relaycommon.RelayInfo, event *dto.OpenAIResponsesWebSocketEvent, rawEvent []byte, roundNumber int64) (*relaycommon.RelayInfo, []byte, *types.NewAPIError) {
	request, err := common.DeepCopy(&event.OpenAIResponsesRequest)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	round := relaycommon.GenRelayInfoResponsesWebSocket(c, request, connectionInfo.ClientWs)
	round.RequestId = responsesWebSocketRoundRequestID(connectionInfo.RequestId, roundNumber)
	round.StartTime = time.Now()
	round.FirstResponseTime = round.StartTime.Add(-time.Second)
	round.InitChannelMeta(c)
	c.Set("image_generation_call", false)
	c.Set("image_generation_call_quality", "")
	c.Set("image_generation_call_size", "")
	billingInput, err := helper.BuildBillingExprRequestInputFromRequest(request, round.RequestHeaders)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	round.BillingRequestInput = &billingInput

	filteredToolTypes := getFilteredResponsesToolTypes(round.ChannelOtherSettings)
	if len(filteredToolTypes) > 0 {
		if err = request.RemoveTools(filteredToolTypes...); err != nil {
			return nil, nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
	}
	if err = helper.ModelMappedHelper(c, round, request); err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	adaptor := GetAdaptor(round.ApiType)
	if adaptor == nil {
		return nil, nil, types.NewError(fmt.Errorf("invalid api type: %d", round.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(round)
	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, round, *request)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(round, converted)
	payload, err := common.Marshal(converted)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	payload, err = relaycommon.RemoveDisabledFields(payload, round.ChannelOtherSettings, false)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(round.ParamOverride) > 0 {
		payload, err = relaycommon.ApplyParamOverrideWithRelayInfo(payload, round)
		if err != nil {
			return nil, nil, newAPIErrorFromParamOverride(err)
		}
	}
	if len(filteredToolTypes) > 0 {
		payload, err = removeResponsesToolsFromBody(payload, filteredToolTypes...)
		if err != nil {
			return nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
	}
	payload, err = mergeResponsesWebSocketEnvelope(payload, rawEvent, event)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	return round, payload, nil
}

func preConsumeResponsesWebSocketRound(c *gin.Context, round *relaycommon.RelayInfo, generate bool) *types.NewAPIError {
	meta := round.Request.GetTokenCountMeta()
	if setting.ShouldCheckPromptSensitive() && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, "Responses WebSocket sensitive words detected: "+strings.Join(words, ", "))
			return types.NewError(errors.New("sensitive words detected"), types.ErrorCodeSensitiveWordsDetected, types.ErrOptionWithSkipRetry())
		}
	}
	tokens, err := service.EstimateRequestToken(c, meta, round)
	if err != nil {
		return types.NewError(err, types.ErrorCodeCountTokenFailed, types.ErrOptionWithSkipRetry())
	}
	round.SetEstimatePromptTokens(tokens)
	priceData, err := helper.ModelPriceHelper(c, round, tokens, meta)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeModelPriceError, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if !generate || priceData.FreeModel {
		return nil
	}
	return service.PreConsumeBilling(c, priceData.QuotaToPreConsume, round)
}

func responsesWebSocketRoundRequestID(base string, roundNumber int64) string {
	suffix := fmt.Sprintf("-%d", roundNumber)
	baseRunes := []rune(base)
	maxBaseRunes := 64 - len([]rune(suffix))
	if maxBaseRunes < 0 {
		maxBaseRunes = 0
	}
	if len(baseRunes) > maxBaseRunes {
		baseRunes = baseRunes[:maxBaseRunes]
	}
	return string(baseRunes) + suffix
}

func mergeResponsesWebSocketEnvelope(converted, rawEvent []byte, event *dto.OpenAIResponsesWebSocketEvent) ([]byte, error) {
	var output map[string]json.RawMessage
	if err := common.Unmarshal(converted, &output); err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := common.Unmarshal(rawEvent, &raw); err != nil {
		return nil, err
	}
	requestFields := responsesRequestJSONFields()
	for key, value := range raw {
		if key == "type" || key == "generate" || key == "stream_id" || key == "background" {
			continue
		}
		if _, knownRequestField := requestFields[key]; knownRequestField {
			continue
		}
		if _, exists := output[key]; !exists {
			output[key] = value
		}
	}
	delete(output, "stream")
	delete(output, "background")
	typeBytes, err := common.Marshal(dto.OpenAIResponsesWebSocketEventTypeCreate)
	if err != nil {
		return nil, err
	}
	output["type"] = typeBytes
	if event.Generate != nil {
		generateBytes, err := common.Marshal(*event.Generate)
		if err != nil {
			return nil, err
		}
		output["generate"] = generateBytes
	}
	if event.StreamID != nil {
		streamIDBytes, err := common.Marshal(*event.StreamID)
		if err != nil {
			return nil, err
		}
		output["stream_id"] = streamIDBytes
	}
	return common.Marshal(output)
}

func responsesRequestJSONFields() map[string]struct{} {
	fields := make(map[string]struct{})
	typeOfRequest := reflect.TypeOf(dto.OpenAIResponsesRequest{})
	for i := 0; i < typeOfRequest.NumField(); i++ {
		name := strings.Split(typeOfRequest.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			fields[name] = struct{}{}
		}
	}
	return fields
}
