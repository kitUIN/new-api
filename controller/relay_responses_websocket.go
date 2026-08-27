package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	responsesWebSocketFirstFrameTimeout = 15 * time.Second
	responsesWebSocketFirstFrameLimit   = 16 << 20
)

var responsesWebSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// API clients generally omit Origin. Browser origin restrictions remain a
		// deployment concern, consistent with the existing Realtime endpoint.
		return true
	},
}

func RelayResponsesWebSocket(c *gin.Context) {
	client, err := responsesWebSocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.LogError(c, "upgrade Responses WebSocket failed: "+err.Error())
		return
	}
	defer client.Close()
	client.SetReadLimit(responsesWebSocketFirstFrameLimit)
	_ = client.SetReadDeadline(time.Now().Add(responsesWebSocketFirstFrameTimeout))
	messageType, firstFrame, err := client.ReadMessage()
	_ = client.SetReadDeadline(time.Time{})
	if err != nil {
		_ = relay.WriteResponsesWebSocketError(c, client, http.StatusBadRequest, string(types.ErrorCodeInvalidRequest), fmt.Errorf("failed to read first response.create event: %w", err))
		return
	}
	if messageType != websocket.TextMessage {
		_ = relay.WriteResponsesWebSocketError(c, client, http.StatusBadRequest, string(types.ErrorCodeInvalidRequest), errors.New("the first WebSocket message must be a text response.create event"))
		return
	}
	event, err := dto.ParseOpenAIResponsesWebSocketEvent(firstFrame)
	if err != nil {
		_ = relay.WriteResponsesWebSocketError(c, client, http.StatusBadRequest, string(types.ErrorCodeInvalidRequest), err)
		return
	}
	if event.Model == "" {
		_ = relay.WriteResponsesWebSocketError(c, client, http.StatusBadRequest, string(types.ErrorCodeInvalidRequest), errors.New("model is required in the first response.create event"))
		return
	}
	hasBackground, err := dto.HasOpenAIResponsesWebSocketBackground(firstFrame)
	if err != nil {
		_ = relay.WriteResponsesWebSocketError(c, client, http.StatusBadRequest, string(types.ErrorCodeInvalidRequest), err)
		return
	}
	if hasBackground {
		_ = relay.WriteResponsesWebSocketError(c, client, http.StatusBadRequest, string(types.ErrorCodeInvalidRequest), errors.New("background is not supported in Responses WebSocket mode"))
		return
	}

	// Distribute normally parses the HTTP body. Supplying the already-read
	// first frame preserves the existing token, group, affinity, and channel
	// selection semantics without consuming the WebSocket frame twice.
	c.Request.Body = io.NopCloser(bytes.NewReader(firstFrame))
	c.Request.ContentLength = int64(len(firstFrame))
	c.Request.Header.Set("Content-Type", "application/json")
	middleware.SetOpenAIAbortHandler(c, func(statusCode int, message string, code string) {
		_ = relay.WriteResponsesWebSocketError(c, client, statusCode, code, errors.New(message))
	})
	middleware.SkipDistributorPostAffinity(c)
	middleware.Distribute()(c)
	if c.IsAborted() {
		return
	}
	common.CleanupBodyStorage(c)
	c.Request.Body = http.NoBody
	c.Request.ContentLength = 0

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIResponsesWebSocket, &event.OpenAIResponsesRequest, client)
	if err != nil {
		_ = relay.WriteResponsesWebSocketError(c, client, http.StatusInternalServerError, string(types.ErrorCodeGenRelayInfoFailed), err)
		return
	}

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: info.TokenGroup,
		ModelName:  info.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	var finalErr *types.NewAPIError
	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		info.RetryIndex = retryParam.GetRetry()
		selectedChannel, channelErr := getChannel(c, info, retryParam)
		if channelErr != nil {
			finalErr = channelErr
			break
		}
		addUsedChannel(c, selectedChannel.Id)
		target, dialErr := relay.DialResponsesWebSocketUpstream(c, info)
		if dialErr == nil {
			info.TargetWs = target
			service.RecordChannelAffinity(c, selectedChannel.Id)
			if runErr := relay.RunResponsesWebSocket(c, info, firstFrame); runErr != nil && !websocket.IsCloseError(runErr, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.LogError(c, "Responses WebSocket relay ended: "+runErr.Error())
			}
			return
		}
		finalErr = dialErr
		info.LastError = dialErr
		if dialErr.GetErrorCode() != types.ErrorCodeChannelResponsesWebSocketUnsupported {
			processChannelError(c, *types.NewChannelError(selectedChannel.Id, selectedChannel.Type, selectedChannel.Name, selectedChannel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), selectedChannel.GetAutoBan()), dialErr, info)
		}
		if !shouldRetry(c, dialErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
		service.PrepareAutoGroupAffinityFailover(c, retryParam)
		retryParam.ExcludeChannel(selectedChannel.Id)
	}
	if finalErr == nil {
		finalErr = types.NewError(errors.New("no Responses WebSocket channel is available"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	statusCode := finalErr.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusBadGateway
	}
	_ = relay.WriteResponsesWebSocketError(c, client, statusCode, string(finalErr.GetErrorCode()), finalErr)
}
