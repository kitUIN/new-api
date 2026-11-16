package router

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageLogStreamPermissionsAndCompression(t *testing.T) {
	for _, tc := range []struct {
		path    string
		role    int
		allowed bool
	}{
		{"/api/log/events", common.RoleCommonUser, false},
		{"/api/log/events", common.RoleAdminUser, true},
		{"/api/log/self/events", common.RoleCommonUser, true},
		{"/api/log/self/events", 0, false},
	} {
		t.Run(tc.path+"/"+common.GetJsonString(tc.role), func(t *testing.T) {
			engine := gin.New()
			engine.Use(sessions.Sessions("test", cookie.NewStore([]byte("usage-log-test-only"))))
			engine.Use(func(c *gin.Context) {
				if tc.role > 0 {
					s := sessions.Default(c)
					s.Set("id", 42)
					s.Set("username", "owner")
					s.Set("role", tc.role)
					s.Set("status", common.UserStatusEnabled)
				}
				c.Next()
			})
			SetApiRouter(engine)
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // permit the first flushed event, then immediately disconnect
			req := httptest.NewRequest("GET", tc.path, nil).WithContext(ctx)
			req.Header.Set("New-Api-User", "42")
			req.Header.Set("Accept-Encoding", "gzip")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			if tc.allowed {
				require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
				require.Empty(t, recorder.Header().Get("Content-Encoding"))
				require.Contains(t, recorder.Body.String(), "event:changed")
			} else {
				require.NotContains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
			}
		})
	}
}
