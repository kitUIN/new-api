package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesHTTPAndWebSocketRoutesCoexist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	routes := engine.Routes()
	found := map[string]bool{}
	for _, route := range routes {
		if route.Path == "/v1/responses" {
			found[route.Method] = true
		}
	}
	require.True(t, found[http.MethodGet])
	require.True(t, found[http.MethodPost])
}
