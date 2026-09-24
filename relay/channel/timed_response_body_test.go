package channel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoRequestSSETimingSeparatesHeadersFirstByteAndEnd(t *testing.T) {
	firstChunk := make(chan struct{})
	lastChunk := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		select {
		case <-firstChunk:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-lastChunk:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "data: last\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	service.InitHttpClient()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{IsStream: true, DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{}}
	resp, err := doRequest(c, req, info)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, header, first, end := info.UpstreamTiming()
	require.False(t, header.IsZero())
	require.True(t, first.IsZero())
	require.True(t, end.IsZero(), "response headers must not end an SSE request")

	close(firstChunk)
	chunk := make([]byte, len("data: first\n\n"))
	_, err = io.ReadFull(resp.Body, chunk)
	require.NoError(t, err)
	_, _, first, end = info.UpstreamTiming()
	require.False(t, first.IsZero())
	require.True(t, end.IsZero(), "receiving the first chunk must not end the request")

	// Model a provider generating subsequent tokens after its first response.
	time.Sleep(20 * time.Millisecond)
	close(lastChunk)
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	_, _, _, end = info.UpstreamTiming()
	require.GreaterOrEqual(t, end.Sub(first).Milliseconds(), int64(20))
	require.NoError(t, resp.Body.Close())
	_, _, _, closedAt := info.UpstreamTiming()
	require.Equal(t, end, closedAt, "closing after EOF must preserve the receipt time")
}

func TestTimedResponseBodyCloseRecordsInterruptedStream(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	info.MarkUpstreamRequestStart()
	info.MarkUpstreamResponseHeader()
	reader, writer := io.Pipe()
	defer writer.Close()
	body := &timedResponseBody{ReadCloser: reader, info: info}
	require.NoError(t, body.Close())
	_, _, first, end := info.UpstreamTiming()
	require.True(t, first.IsZero(), "a stream with no body must not invent a first byte")
	require.False(t, end.IsZero())
	_, err := writer.Write([]byte("late"))
	require.ErrorIs(t, err, io.ErrClosedPipe, "Close must reach the underlying body")
	info.MarkUpstreamRequestStart()
	_, header, first, end := info.UpstreamTiming()
	require.True(t, header.IsZero())
	require.True(t, first.IsZero())
	require.True(t, end.IsZero(), "a retry must reset upstream timing")
}
