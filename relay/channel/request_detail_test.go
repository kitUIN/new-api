package channel

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoRequestCapturesResponseWithoutChangingDelivery(t *testing.T) {
	previousMode := common.LogRequestDetailMode.Load()
	t.Cleanup(func() { common.LogRequestDetailMode.Store(previousMode) })
	service.InitHttpClient()
	for _, mode := range []int32{common.RequestDetailLogOff, common.RequestDetailLogFailed, common.RequestDetailLogAll} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%d/stream=%t", mode, stream), func(t *testing.T) {
				common.LogRequestDetailMode.Store(mode)
				response := `{"result":"ok"}`
				contentType := "application/json"
				if stream {
					response = "data: {\"result\":\"ok\"}\n\ndata: [DONE]\n\n"
					contentType = "text/event-stream"
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", contentType)
					_, _ = io.WriteString(w, response)
				}))
				defer server.Close()
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
				req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
				require.NoError(t, err)
				info := &relaycommon.RelayInfo{IsStream: stream, DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{}}
				resp, err := doRequest(ctx, req, info)
				require.NoError(t, err)
				defer resp.Body.Close()
				value, captured := ctx.Get("upstream_response_body_buf")
				if mode == common.RequestDetailLogOff {
					require.False(t, captured)
					_, hasHeaders := ctx.Get("upstream_request_headers")
					require.False(t, hasHeaders)
				} else {
					require.True(t, captured)
					require.Empty(t, value.(*common.RequestDetailBodyCapture).String(), "capturing must not eagerly read the response")
				}
				delivered, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, response, string(delivered))
				if captured {
					require.Equal(t, response, value.(*common.RequestDetailBodyCapture).String())
				}
			})
		}
	}
}
