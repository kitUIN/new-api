package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRefreshLogsRejectsMalformedIDs(t *testing.T) {
	for _, ids := range []string{"", "0", "-1", "1,no", "1,", strings.Repeat("1,", 1000) + "1"} {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/log/refresh?ids="+ids, nil)
		RefreshAllLogs(c)
		require.Equal(t, http.StatusBadRequest, response.Code)
	}
}

func TestRefreshUserLogsNeverUsesAdminScopeWithoutUser(t *testing.T) {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/log/self/refresh?ids=1", nil)
	RefreshUserLogs(c)
	require.Equal(t, http.StatusUnauthorized, response.Code)
}
