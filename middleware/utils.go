package middleware

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const openAIAbortHandlerContextKey = "openai_abort_handler"
const skipDistributorPostAffinityContextKey = "skip_distributor_post_affinity"

type OpenAIAbortHandler func(statusCode int, message string, code string)

func SetOpenAIAbortHandler(c *gin.Context, handler OpenAIAbortHandler) {
	c.Set(openAIAbortHandlerContextKey, handler)
}

func SkipDistributorPostAffinity(c *gin.Context) {
	c.Set(skipDistributorPostAffinityContextKey, true)
}

func abortWithOpenAiMessage(c *gin.Context, statusCode int, message string, code ...types.ErrorCode) {
	codeStr := ""
	if len(code) > 0 {
		codeStr = string(code[0])
	}
	responseMessage := types.LimitErrorMessageForResponse(common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)))
	if value, exists := c.Get(openAIAbortHandlerContextKey); exists {
		if handler, ok := value.(OpenAIAbortHandler); ok && handler != nil {
			handler(statusCode, responseMessage, codeStr)
			c.Abort()
			logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", c.GetInt("id"), message))
			return
		}
	}
	userId := c.GetInt("id")
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": responseMessage,
			"type":    "new_api_error",
			"code":    codeStr,
		},
	})
	c.Abort()
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, message))
}

func abortWithMidjourneyMessage(c *gin.Context, statusCode int, code int, description string) {
	c.JSON(statusCode, gin.H{
		"description": types.LimitErrorMessageForResponse(description),
		"type":        "new_api_error",
		"code":        code,
	})
	c.Abort()
	logger.LogError(c.Request.Context(), description)
}
