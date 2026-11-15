package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type responsesFlushRecorder struct {
	*httptest.ResponseRecorder
	onFlush func()
}

func (w *responsesFlushRecorder) Flush() {
	w.ResponseRecorder.Flush()
	if w.onFlush != nil {
		w.onFlush()
	}
}

// Keep the upstream body open after the event. A terminal event or client
// cancellation must finish the relay without waiting for an upstream EOF.
func runResponsesStreamEvent(t *testing.T, event string, cancelOnFlush bool) (*httptest.ResponseRecorder, *relaycommon.RelayInfo, *dto.Usage, *types.NewAPIError, relaycommon.StreamEndReason) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		cancel()
		_ = reader.Close()
		_ = writer.Close()
	})
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
		DisablePing: true,
		IsStream:    true,
	}
	recorder := httptest.NewRecorder()
	var reasonAtFlush relaycommon.StreamEndReason
	responseWriter := &responsesFlushRecorder{ResponseRecorder: recorder}
	responseWriter.onFlush = func() {
		reasonAtFlush = info.StreamStatus.EndReason
		if cancelOnFlush {
			cancel()
		}
	}
	c, _ := gin.CreateTestContext(responseWriter)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	info.MarkUpstreamRequestStart()

	go func() {
		_, _ = fmt.Fprintf(writer, "data: %s\n\n", event)
	}()
	var usage *dto.Usage
	var streamErr *types.NewAPIError
	done := make(chan struct{})
	go func() {
		usage, streamErr = OaiResponsesStreamHandler(c, info, &http.Response{Body: reader})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		cancel()
		_ = reader.Close()
		_ = writer.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("stream did not stop after closing the upstream body")
		}
		t.Fatal("stream waited for upstream EOF instead of finishing promptly")
	}
	return recorder, info, usage, streamErr, reasonAtFlush
}

func TestOaiResponsesStreamHandler_TerminalEvent(t *testing.T) {
	for _, eventType := range []string{"response.completed", "response.incomplete"} {
		for _, cancelOnFlush := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cancel_on_flush=%t", eventType, cancelOnFlush), func(t *testing.T) {
				event := fmt.Sprintf(`{"type":%q,"response":{"usage":{"input_tokens":120,"output_tokens":7,"total_tokens":127,"input_tokens_details":{"cached_tokens":100}}}}`, eventType)
				recorder, info, usage, streamErr, reasonAtFlush := runResponsesStreamEvent(t, event, cancelOnFlush)

				require.Nil(t, streamErr)
				require.NotNil(t, usage)
				require.Equal(t, 120, usage.PromptTokens)
				require.Equal(t, 7, usage.CompletionTokens)
				require.Equal(t, 127, usage.TotalTokens)
				require.Equal(t, 100, usage.PromptTokensDetails.CachedTokens)
				require.Contains(t, recorder.Body.String(), "event: "+eventType)
				require.Contains(t, recorder.Body.String(), "data: "+event)
				require.Equal(t, relaycommon.StreamEndReasonDone, reasonAtFlush, "completion must be recorded before the client receives the terminal event")
				require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
				require.Nil(t, info.StreamStatus.EndError)
				require.False(t, info.StreamStatus.HasErrors())
				_, _, _, end := info.UpstreamTiming()
				require.False(t, end.IsZero(), "upstream receipt must finish before the relay returns")
			})
		}
	}
}

func TestOaiResponsesStreamHandler_ClientGoneBeforeTerminal(t *testing.T) {
	event := `{"type":"response.created","response":{"id":"resp_test"}}`
	recorder, info, usage, streamErr, reasonAtFlush := runResponsesStreamEvent(t, event, true)

	require.Nil(t, streamErr)
	require.NotNil(t, usage)
	require.Zero(t, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), event)
	require.Equal(t, relaycommon.StreamEndReasonNone, reasonAtFlush)
	require.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
	require.ErrorIs(t, info.StreamStatus.EndError, context.Canceled)
}

func TestOaiResponsesStreamHandler_UpstreamFailure(t *testing.T) {
	event := `{"type":"response.failed","response":{"error":{"type":"server_error","code":"upstream_failed","message":"generation failed"}}}`
	recorder, info, usage, streamErr, _ := runResponsesStreamEvent(t, event, false)

	require.Nil(t, usage)
	require.NotNil(t, streamErr)
	require.Contains(t, streamErr.Error(), "generation failed")
	require.Empty(t, recorder.Body.String())
	require.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	require.True(t, info.StreamStatus.HasErrors())
}
