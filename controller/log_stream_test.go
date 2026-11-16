package controller

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageLogStreamFlushesBeforeRequestCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/events", func(c *gin.Context) { streamUsageLogs(c, 42) })
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/events", nil)
	require.NoError(t, err)
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))
	require.Equal(t, "no", response.Header.Get("X-Accel-Buffering"))
	reader := bufio.NewReader(response.Body)
	readEvent := func() string {
		var data strings.Builder
		for {
			line, err := reader.ReadString('\n')
			require.NoError(t, err)
			data.WriteString(line)
			if line == "\n" {
				return data.String()
			}
		}
	}
	require.Contains(t, readEvent(), "event:changed")
	model.NotifyUsageLogChanged(42)
	require.Contains(t, readEvent(), "event:changed")
}
