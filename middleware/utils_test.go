package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAbortWithOpenAIMessageLimitsResponseOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	message := strings.Repeat("错误", types.MaxErrorMessageRunes)
	abortWithOpenAiMessage(c, http.StatusBadRequest, message)

	var response struct {
		Error types.OpenAIError `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, types.MaxErrorMessageRunes, utf8.RuneCountInString(response.Error.Message))
	require.True(t, strings.HasSuffix(response.Error.Message, "..."))
}

func TestAbortWithMidjourneyMessageLimitsResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/mj/submit", nil)

	description := strings.Repeat("错误", types.MaxErrorMessageRunes+1)
	abortWithMidjourneyMessage(c, http.StatusBadRequest, 1, description)

	var response struct {
		Description string `json:"description"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, types.MaxErrorMessageRunes, utf8.RuneCountInString(response.Description))
	require.True(t, strings.HasSuffix(response.Description, "..."))
}
