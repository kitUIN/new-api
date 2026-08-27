package dto

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const OpenAIResponsesWebSocketEventTypeCreate = "response.create"

// OpenAIResponsesWebSocketEvent is a Responses request plus the WebSocket
// envelope fields. Pointer scalars preserve explicit false and zero values.
type OpenAIResponsesWebSocketEvent struct {
	OpenAIResponsesRequest
	Type     string  `json:"type"`
	Generate *bool   `json:"generate,omitempty"`
	StreamID *string `json:"stream_id,omitempty"`
}

func ParseOpenAIResponsesWebSocketEvent(data []byte) (*OpenAIResponsesWebSocketEvent, error) {
	var event OpenAIResponsesWebSocketEvent
	if err := common.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("invalid response.create event: %w", err)
	}
	if strings.TrimSpace(event.Type) != OpenAIResponsesWebSocketEventTypeCreate {
		return nil, fmt.Errorf("event type must be %q", OpenAIResponsesWebSocketEventTypeCreate)
	}
	return &event, nil
}

func HasOpenAIResponsesWebSocketBackground(data []byte) (bool, error) {
	var envelope map[string]json.RawMessage
	if err := common.Unmarshal(data, &envelope); err != nil {
		return false, err
	}
	raw, ok := envelope["background"]
	if !ok {
		return false, nil
	}
	var background bool
	if err := common.Unmarshal(raw, &background); err != nil {
		return false, fmt.Errorf("background must be a boolean")
	}
	return true, nil
}

type OpenAIResponsesWebSocketErrorEvent struct {
	Type   string `json:"type"`
	Status int    `json:"status"`
	Error  struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func NewOpenAIResponsesWebSocketErrorEvent(status int, code, message string) OpenAIResponsesWebSocketErrorEvent {
	event := OpenAIResponsesWebSocketErrorEvent{Type: "error", Status: status}
	event.Error.Code = code
	event.Error.Message = message
	return event
}
