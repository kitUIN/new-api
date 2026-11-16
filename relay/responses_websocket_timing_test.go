package relay

import (
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestResponsesWebSocketTimingPerRound(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	// A persistent connection already has its own handshake timing. None of
	// those timestamps should appear in a response.create consumption log.
	s.connectionInfo.MarkUpstreamRequestStart()
	s.connectionInfo.MarkUpstreamResponseHeader()
	s.connectionInfo.MarkUpstreamFirstByte()
	s.connectionInfo.MarkUpstreamRequestEnd()

	for i, terminal := range []string{"response.completed", "response.incomplete"} {
		beforeCreate := time.Now()
		require.Nil(t, s.handleClientFrame(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-4o","input":"Hello"}`)))
		readSteeringFrame(t, provider)
		round := s.round
		start, header, first, end := round.UpstreamTiming()
		require.False(t, start.Before(beforeCreate))
		require.True(t, header.IsZero())
		require.True(t, first.IsZero())
		require.True(t, end.IsZero())

		beforeFirst := time.Now()
		sendSteeringUpstreamEvent(t, s, client, fmt.Sprintf(`{"type":"response.created","response":{"id":"resp_%d"}}`, i+1))
		_, _, first, end = round.UpstreamTiming()
		require.False(t, first.Before(beforeFirst))
		require.False(t, first.After(time.Now()))
		require.True(t, end.IsZero())
		require.False(t, round.HasSendResponse(), "lifecycle events must not count as downstream TTFT")

		sendSteeringUpstreamEvent(t, s, client, `{"type":"response.in_progress"}`)
		// Simulate generation after the lifecycle event, including on systems
		// whose clock resolution can make immediate events share a timestamp.
		time.Sleep(5 * time.Millisecond)
		sendSteeringUpstreamEvent(t, s, client, `{"type":"response.output_text.delta","delta":"Hello"}`)
		require.True(t, round.HasSendResponse())
		require.False(t, round.FirstResponseTime.Before(first))
		_, _, repeatedFirst, _ := round.UpstreamTiming()
		require.Equal(t, first, repeatedFirst)

		// A terminal event for another response must not end the active round.
		sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"unrelated"}}`)
		_, _, _, end = round.UpstreamTiming()
		require.True(t, end.IsZero())
		require.Same(t, round, s.round)

		beforeTerminal := time.Now()
		sendSteeringUpstreamEvent(t, s, client, fmt.Sprintf(`{"type":%q,"response":{"id":"resp_%d","usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`, terminal, i+1))
		_, _, _, end = round.UpstreamTiming()
		require.False(t, end.Before(beforeTerminal))
		require.False(t, end.After(time.Now()))
		require.Nil(t, s.round)
	}
	// Verify the persisted log payload, not just the in-memory timestamps.
	assertSteeringBilling(t, db, []int{20, 20}, []int{3, 3})
}

func TestResponsesWebSocketTimingEndsOnFailure(t *testing.T) {
	for _, event := range []string{"response.failed", "error", "disconnect"} {
		t.Run(event, func(t *testing.T) {
			s, client, provider, db := newSteeringTestSession(t)
			startSteeringTestRound(t, s, client, provider)
			round := s.round
			beforeEnd := time.Now()
			if event == "disconnect" {
				s.refundRound()
			} else {
				sendSteeringUpstreamEvent(t, s, client, fmt.Sprintf(`{"type":%q}`, event))
			}
			_, _, _, end := round.UpstreamTiming()
			require.False(t, end.Before(beforeEnd))
			require.False(t, end.After(time.Now()))
			require.Nil(t, s.round)
			assertSteeringBilling(t, db, nil, nil)
		})
	}
}
