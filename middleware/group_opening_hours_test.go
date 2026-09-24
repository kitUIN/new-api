package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDistributeRejectsClosedGroupBeforeSelectingChannel(t *testing.T) {
	config := ratio_setting.GetGroupRatioSetting().GroupOpeningHours
	original, err := common.Marshal(config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, common.Unmarshal(original, config)) })
	now := time.Now().In(time.FixedZone("UTC+8", 8*3600))
	start, end := now.Add(time.Hour).Format("15:04"), now.Add(2*time.Hour).Format("15:04")
	value, err := common.Marshal(map[string][]ratio_setting.GroupOpeningInterval{"closed-test": {{Start: start, End: end}}})
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(value, config))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "closed-test")
	// Even an explicit channel must not bypass the schedule.
	common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, "1")
	Distribute()(c)
	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "group_not_open")
	require.Contains(t, w.Body.String(), "未开放")
	require.Contains(t, w.Body.String(), start)
	require.Contains(t, w.Body.String(), end)
}
