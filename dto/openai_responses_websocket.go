package dto

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	OpenAIResponsesWebSocketEventTypeCreate = "response.create"
	OpenAIResponsesWebSocketEventTypeSteer  = "response.steer"
)

// Steering is a control event, not a new create request. In particular, it
// must not acquire a model, stream_id or channel parameter overrides.
type OpenAIResponsesWebSocketSteerEvent struct {
	Type               string          `json:"type"`
	PreviousResponseID string          `json:"previous_response_id"`
	Input              json.RawMessage `json:"input"`
}

type OpenAIResponsesWebSocketSteer struct {
	ID                 string          `json:"id,omitempty"`
	PreviousResponseID string          `json:"previous_response_id"`
	Input              json.RawMessage `json:"input,omitempty"`
}

type OpenAIResponsesWebSocketServerEvent struct {
	ResponsesStreamResponse
	Steer  *OpenAIResponsesWebSocketSteer `json:"steer,omitempty"`
	Reason string                         `json:"reason,omitempty"`
}

func ParseOpenAIResponsesWebSocketSteerEvent(data []byte) (*OpenAIResponsesWebSocketSteerEvent, error) {
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("invalid response.steer event: %w", err)
	}
	for key := range fields {
		if key != "type" && key != "previous_response_id" && key != "input" {
			return nil, fmt.Errorf("response.steer does not support field %q", key)
		}
	}
	var event OpenAIResponsesWebSocketSteerEvent
	if err := common.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("invalid response.steer event: %w", err)
	}
	if event.Type != OpenAIResponsesWebSocketEventTypeSteer || strings.TrimSpace(event.PreviousResponseID) == "" {
		return nil, fmt.Errorf("response.steer requires type and previous_response_id")
	}
	if _, err := ResponsesWebSocketInputItems(event.Input); err != nil {
		return nil, err
	}
	if common.GetJsonType(event.Input) == "array" {
		var messages []map[string]json.RawMessage
		if err := common.Unmarshal(event.Input, &messages); err != nil || len(messages) == 0 {
			return nil, fmt.Errorf("response.steer input must be a string or a nonempty array of user messages")
		}
		for _, message := range messages {
			for key := range message {
				if key != "type" && key != "role" && key != "content" {
					return nil, fmt.Errorf("response.steer message does not support field %q", key)
				}
			}
			var role, messageType string
			_ = common.Unmarshal(message["role"], &role)
			if rawType, ok := message["type"]; ok {
				if err := common.Unmarshal(rawType, &messageType); err != nil || messageType != "message" {
					return nil, fmt.Errorf("response.steer input only supports user messages")
				}
			}
			if role != "user" {
				return nil, fmt.Errorf("response.steer input only supports user messages")
			}
			switch common.GetJsonType(message["content"]) {
			case "string":
			case "array":
				var parts []struct {
					Type string `json:"type"`
				}
				if err := common.Unmarshal(message["content"], &parts); err != nil {
					return nil, fmt.Errorf("invalid response.steer message content")
				}
				for _, part := range parts {
					if part.Type != "input_text" && part.Type != "input_image" && part.Type != "input_file" {
						return nil, fmt.Errorf("unsupported response.steer content type %q", part.Type)
					}
				}
			default:
				return nil, fmt.Errorf("response.steer message content must be a string or an array")
			}
		}
	}
	return &event, nil
}

// ResponsesWebSocketInputItems normalizes input for billing only. Accepted
// steering must never be replayed in the upstream response.create payload.
func ResponsesWebSocketInputItems(input json.RawMessage) ([]json.RawMessage, error) {
	switch common.GetJsonType(input) {
	case "string":
		message, err := common.Marshal(map[string]any{"role": "user", "content": input})
		return []json.RawMessage{message}, err
	case "array":
		var items []json.RawMessage
		err := common.Unmarshal(input, &items)
		return items, err
	default:
		return nil, fmt.Errorf("input must be a string or an array")
	}
}

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
