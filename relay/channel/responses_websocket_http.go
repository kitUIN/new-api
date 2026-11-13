package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// UseResponsesWebSocket selects the upstream transport independently of the
// client's transport. Only channels with a Responses-compatible adapter qualify.
func UseResponsesWebSocket(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil || !info.ChannelOtherSettings.UseResponsesWebSocket {
		return false
	}
	switch info.ChannelType {
	case constant.ChannelTypeOpenAI, constant.ChannelTypeAzure, constant.ChannelTypeCodex:
		return true
	default:
		return false
	}
}

func responsesWebSocketPayload(requestBody io.Reader) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := common.DecodeJson(requestBody, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, errors.New("Responses request must be a JSON object")
	}
	if raw, ok := payload["background"]; ok {
		var background bool
		if err := common.Unmarshal(raw, &background); err != nil || background {
			return nil, errors.New("background generation is not supported with upstream Responses WebSocket")
		}
	}
	delete(payload, "background")
	delete(payload, "stream")
	delete(payload, "stream_options")
	payload["type"] = json.RawMessage(`"response.create"`)
	return common.Marshal(payload)
}

func doResponsesWebSocketRequest(a Adaptor, c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (httpResponse *http.Response, requestErr error) {
	defer func() {
		if requestErr != nil || (httpResponse != nil && httpResponse.StatusCode != http.StatusOK) {
			info.MarkUpstreamRequestEnd()
		}
		if httpResponse != nil && httpResponse.Body != nil && common.IsRequestDetailLoggingEnabled() {
			var capture common.RequestDetailBodyCapture
			httpResponse.Body = struct {
				io.Reader
				io.Closer
			}{io.TeeReader(httpResponse.Body, &capture), httpResponse.Body}
			c.Set("upstream_response_body_buf", &capture)
		}
	}()
	payload, err := responsesWebSocketPayload(requestBody)
	if err != nil {
		return responsesWebSocketHTTPError(http.StatusBadRequest, "invalid_request_error", err.Error()), nil
	}
	info.UpstreamTransport = types.RelayTransportWebSocket
	conn, handshake, err := dialWebSocket(a, c, info)
	if err != nil {
		// Preserve HTTP errors (including 401/429 and Retry-After) so the usual
		// channel retry, status mapping and error handling remain effective.
		if handshake != nil && handshake.StatusCode != http.StatusSwitchingProtocols {
			return handshake, nil
		}
		if handshake != nil && handshake.Body != nil {
			_ = handshake.Body.Close()
		}
		return nil, err
	}
	body := newResponsesWebSocketBody(c.Request.Context(), conn)
	_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err = conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		_ = body.Close()
		return nil, fmt.Errorf("write Responses WebSocket request: %w", err)
	}
	// Read the first event before committing downstream headers. A first-frame
	// error can still be returned as a regular HTTP error and retried safely.
	data, event, err := body.readEvent()
	if err != nil {
		_ = body.Close()
		return nil, err
	}
	info.MarkUpstreamFirstByte()
	if response := responsesWebSocketEventError(event); response != nil {
		_ = body.Close()
		return response, nil
	}
	if info.IsStream {
		body.queueEvent(data, event)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/event-stream"}},
			Body:       body,
		}, nil
	}
	defer body.Close()
	defer info.MarkUpstreamRequestEnd()
	for {
		if response := responsesWebSocketEventError(event); response != nil {
			return response, nil
		}
		if isResponsesWebSocketTerminal(event.Type) {
			// Keep the original response object, including fields unknown to our DTO.
			var envelope struct {
				Response json.RawMessage `json:"response"`
			}
			if err = common.Unmarshal(data, &envelope); err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(envelope.Response))}, nil
		}
		data, event, err = body.readEvent()
		if err != nil {
			return nil, err
		}
	}
}

func responsesWebSocketHTTPError(status int, code, message string) *http.Response {
	payload, _ := common.Marshal(map[string]any{"error": types.OpenAIError{Type: code, Code: code, Message: message}})
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(payload))}
}

func responsesWebSocketEventError(event dto.ResponsesStreamResponse) *http.Response {
	if event.Type != "error" && event.Type != "response.failed" && event.Type != "response.error" {
		return nil
	}
	upstreamError := event.Error
	if upstreamError == nil && event.Response != nil {
		upstreamError = event.Response.GetOpenAIError()
	}
	if upstreamError == nil {
		upstreamError = &types.OpenAIError{Code: event.Code, Message: event.Message}
	}
	if upstreamError.Type == "" {
		upstreamError.Type = "upstream_error"
	}
	if upstreamError.Message == "" {
		upstreamError.Message = "upstream Responses WebSocket returned " + event.Type
	}
	status := event.Status
	if status < 400 || status > 599 {
		status = http.StatusBadGateway
	}
	payload, _ := common.Marshal(map[string]any{"error": upstreamError})
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(payload))}
}

func isResponsesWebSocketTerminal(eventType string) bool {
	switch eventType {
	case "response.completed", "response.incomplete", "response.failed", "response.error", "error":
		return true
	default:
		return false
	}
}

// responsesWebSocketBody exposes one WebSocket response as an SSE response body.
// Reads provide backpressure; Close and context cancellation interrupt ReadMessage.
type responsesWebSocketBody struct {
	conn        *websocket.Conn
	cancel      context.CancelFunc
	closeOnce   sync.Once
	pending     []byte
	terminal    bool
	idleTimeout time.Duration
}

func newResponsesWebSocketBody(parent context.Context, conn *websocket.Conn) *responsesWebSocketBody {
	var ctx context.Context
	var cancel context.CancelFunc
	if common.RelayTimeout > 0 {
		ctx, cancel = context.WithTimeout(parent, time.Duration(common.RelayTimeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	idleTimeout := time.Duration(constant.StreamingTimeout) * time.Second
	if idleTimeout <= 0 {
		idleTimeout = helper.DefaultStreamingTimeout
	}
	body := &responsesWebSocketBody{conn: conn, cancel: cancel, idleTimeout: idleTimeout}
	conn.SetReadLimit(16 << 20)
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		defer conn.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	return body
}

func (b *responsesWebSocketBody) Close() error {
	b.closeOnce.Do(func() { b.cancel(); _ = b.conn.Close() })
	return nil
}

func (b *responsesWebSocketBody) readEvent() ([]byte, dto.ResponsesStreamResponse, error) {
	var event dto.ResponsesStreamResponse
	_ = b.conn.SetReadDeadline(time.Now().Add(b.idleTimeout))
	kind, data, err := b.conn.ReadMessage()
	if err != nil {
		return nil, event, fmt.Errorf("upstream Responses WebSocket ended before a terminal event: %w", err)
	}
	if kind != websocket.TextMessage {
		return nil, event, errors.New("upstream Responses WebSocket returned a non-text message")
	}
	if err = common.Unmarshal(data, &event); err != nil || event.Type == "" {
		return nil, event, errors.New("upstream Responses WebSocket returned an invalid event")
	}
	if (event.Type == "response.completed" || event.Type == "response.incomplete") && event.Response == nil {
		return nil, event, errors.New("upstream Responses WebSocket terminal event has no response")
	}
	// WebSocket messages can contain pretty-printed JSON. SSE requires each
	// JSON payload on one data line for the existing stream scanner.
	// Use RawMessage via the common wrapper rather than changing numeric values.
	encoded, err := common.Marshal(json.RawMessage(data))
	if err != nil {
		return nil, event, err
	}
	return encoded, event, nil
}

func (b *responsesWebSocketBody) queueEvent(data []byte, event dto.ResponsesStreamResponse) {
	b.pending = append([]byte("data: "), data...)
	b.pending = append(b.pending, '\n', '\n')
	b.terminal = isResponsesWebSocketTerminal(event.Type)
	if b.terminal {
		_ = b.Close()
	}
}

func (b *responsesWebSocketBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(b.pending) == 0 {
		if b.terminal {
			return 0, io.EOF
		}
		data, event, err := b.readEvent()
		if err != nil {
			_ = b.Close()
			return 0, err
		}
		b.queueEvent(data, event)
	}
	n := copy(p, b.pending)
	b.pending = b.pending[n:]
	return n, nil
}
