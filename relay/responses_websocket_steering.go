package relay

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gorilla/websocket"
)

// Only the current (or most recently completed) response can be steered on
// this single-active-response relay. Keep its original request settings for
// automatic successors, without retaining the entire conversation history.
type responsesWebSocketSteering struct {
	request         *dto.OpenAIResponsesWebSocketEvent
	responseID      string
	pending         []dto.OpenAIResponsesWebSocketSteer
	waitingForInput bool
}

func (s *responsesWebSocketSession) handleSteer(data []byte) *types.NewAPIError {
	event, err := dto.ParseOpenAIResponsesWebSocketSteerEvent(data)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if s.steering.request == nil || event.PreviousResponseID != s.steering.responseID {
		return types.NewErrorWithStatusCode(errors.New("response.steer must target the latest response.created on this connection"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	// Bound unacknowledged submissions as well as accepted input. The upstream
	// may enforce a smaller limit and return response.steer.failed normally.
	pendingBytes := len(event.Input)
	for _, pending := range s.steering.pending {
		pendingBytes += len(pending.Input)
	}
	if len(s.steering.pending) >= 128 || pendingBytes > responsesWebSocketMaxMessageBytes {
		return types.NewErrorWithStatusCode(errors.New("too much pending steering input on this connection"), types.ErrorCodeInvalidRequest, http.StatusTooManyRequests, types.ErrOptionWithSkipRetry())
	}
	request := &dto.OpenAIResponsesRequest{Input: event.Input}
	if apiErr := checkResponsesWebSocketSensitive(s.c, request.GetTokenCountMeta()); apiErr != nil {
		return apiErr
	}
	_ = s.upstream.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := s.upstream.WriteMessage(websocket.TextMessage, data); err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeChannelResponsesWebSocketDialFailed, http.StatusBadGateway)
	}
	s.steering.pending = append(s.steering.pending, dto.OpenAIResponsesWebSocketSteer{
		PreviousResponseID: event.PreviousResponseID,
		Input:              event.Input,
	})
	return nil
}

func (s *responsesWebSocketSession) observeSteeringEvent(event dto.OpenAIResponsesWebSocketServerEvent) {
	if event.Steer == nil || event.Steer.PreviousResponseID != s.steering.responseID {
		return
	}
	switch event.Type {
	case "response.steer.accepted":
		if event.Steer.ID == "" {
			return
		}
		for _, pending := range s.steering.pending {
			if pending.ID == event.Steer.ID {
				return
			}
		}
		for i := range s.steering.pending {
			if s.steering.pending[i].ID == "" {
				s.steering.pending[i].ID = event.Steer.ID
				return
			}
		}
	case "response.steer.pending":
		if event.Reason == "waiting_for_required_input" {
			s.steering.waitingForInput = true
		}
	case "response.steer.failed":
		for i, pending := range s.steering.pending {
			if pending.ID == event.Steer.ID {
				// Pre-acceptance failures have no ID; acknowledgements for
				// submissions to the same response arrive in submission order.
				copy(s.steering.pending[i:], s.steering.pending[i+1:])
				s.steering.pending[len(s.steering.pending)-1] = dto.OpenAIResponsesWebSocketSteer{}
				s.steering.pending = s.steering.pending[:len(s.steering.pending)-1]
				return
			}
		}
	}
}

func (s *responsesWebSocketSteering) continuationInput(extra json.RawMessage) (json.RawMessage, error) {
	var input []json.RawMessage
	for _, pending := range s.pending {
		items, err := dto.ResponsesWebSocketInputItems(pending.Input)
		if err != nil {
			return nil, err
		}
		input = append(input, items...)
	}
	if len(extra) > 0 {
		items, err := dto.ResponsesWebSocketInputItems(extra)
		if err != nil {
			return nil, err
		}
		input = append(input, items...)
	}
	return common.Marshal(input)
}

func (s *responsesWebSocketSession) observeResponseCreated(event dto.ResponsesStreamResponse) *types.NewAPIError {
	if event.Type != "response.created" || event.Response == nil || event.Response.ID == "" {
		return nil
	}
	if s.round != nil {
		if s.steering.responseID != "" && s.steering.responseID != event.Response.ID {
			return types.NewError(errors.New("upstream started overlapping Responses WebSocket responses"), types.ErrorCodeBadResponse)
		}
	} else {
		if s.steering.request == nil || len(s.steering.pending) == 0 {
			return types.NewError(errors.New("upstream started an unexpected Responses WebSocket response"), types.ErrorCodeBadResponse)
		}
		var parentID string
		_ = common.Unmarshal(event.Response.PreviousResponseID, &parentID)
		if parentID != "" && parentID != s.steering.responseID {
			return types.NewError(errors.New("upstream steering continuation has an unexpected previous_response_id"), types.ErrorCodeBadResponse)
		}
		request, err := common.DeepCopy(s.steering.request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		request.PreviousResponseID = s.steering.responseID
		request.Generate = nil
		request.Input, err = s.steering.continuationInput(nil)
		if err != nil {
			return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		raw, err := common.Marshal(request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		// The server has already created the successor: initialize a fresh
		// billing session, never send a second response.create upstream. If
		// admission fails, the caller closes both sockets immediately.
		if _, apiErr := s.startRound(request, raw, false); apiErr != nil {
			return apiErr
		}
	}
	s.steering.responseID = event.Response.ID
	return nil
}

func responsesWebSocketNeedsClientInput(response *dto.OpenAIResponsesResponse) bool {
	if response == nil {
		return false
	}
	for _, item := range response.Output {
		switch item.Type {
		case "function_call", "custom_tool_call", "computer_call", "local_shell_call", "shell_call", "apply_patch_call", "mcp_approval_request":
			return true
		}
	}
	return false
}
