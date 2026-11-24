package updatemaint

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestMaintenanceNotice(t *testing.T) {
	deadline := time.Now().In(time.FixedZone("HKT", 8*60*60)).Add(3 * time.Minute)
	n := newNotice(deadline)
	message := "系统正在升级，预计" + deadline.Format(time.DateTime) + "完成，请等待升级完成后再重试"
	for _, path := range []string{"/", "/api/status", "/v1/chat/completions", "/v1beta/models"} {
		response := httptest.NewRecorder()
		n.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		require.Equal(t, deadline.UTC().Format(http.TimeFormat), response.Header().Get("Retry-After"))
		if path == "/" {
			require.Equal(t, message, response.Body.String())
			continue
		}
		var payload struct {
			Message string
			Error   struct{ Message string }
		}
		require.NoError(t, common.DecodeJson(response.Body, &payload))
		require.Equal(t, message, payload.Message)
		require.Equal(t, message, payload.Error.Message)
	}
	expired := httptest.NewRecorder()
	newNotice(deadline.Add(-time.Hour)).ServeHTTP(expired, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, "5", expired.Header().Get("Retry-After"))
}
