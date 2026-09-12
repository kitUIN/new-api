package router

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestTicketRouteRateLimitIsolation(t *testing.T) {
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Ticket{}, &model.TicketMessage{}, &model.TicketAttachment{}, &model.TicketRead{}))
	oldRedis, oldRDB := common.RedisEnabled, common.RDB
	oldGlobal, oldGlobalNum, oldGlobalDuration := common.GlobalApiRateLimitEnable, common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration
	oldCritical, oldCriticalNum := common.CriticalRateLimitEnable, common.CriticalRateLimitNum
	oldTicket, oldAttachment := common.TicketRateLimitNum, common.TicketAttachmentRateLimitNum
	oldQQAddress := common.QQCallbackAddress
	t.Cleanup(func() {
		common.RedisEnabled, common.RDB = oldRedis, oldRDB
		common.GlobalApiRateLimitEnable, common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration = oldGlobal, oldGlobalNum, oldGlobalDuration
		common.CriticalRateLimitEnable, common.CriticalRateLimitNum = oldCritical, oldCriticalNum
		common.TicketRateLimitNum, common.TicketAttachmentRateLimitNum = oldTicket, oldAttachment
		common.QQCallbackAddress = oldQQAddress
	})
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	common.RedisEnabled, common.RDB = true, rdb
	common.GlobalApiRateLimitEnable, common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration = true, 180, 180
	common.CriticalRateLimitEnable, common.CriticalRateLimitNum = true, 1
	common.TicketRateLimitNum, common.TicketAttachmentRateLimitNum = 2, 240
	common.QQCallbackAddress = ""

	engine := gin.New()
	engine.Use(sessions.Sessions("ticket-test", cookie.NewStore([]byte("ticket-test-only"))))
	engine.Use(func(c *gin.Context) {
		id, _ := strconv.Atoi(c.GetHeader("Test-User"))
		if id > 0 {
			session := sessions.Default(c)
			session.Set("id", id)
			session.Set("username", fmt.Sprintf("user-%d", id))
			session.Set("role", common.RoleCommonUser)
			session.Set("status", common.UserStatusEnabled)
		}
		c.Next()
	})
	SetApiRouter(engine)
	request := func(method, path string, userID int) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("title", "ticket"))
		require.NoError(t, writer.WriteField("content", "message"))
		require.NoError(t, writer.Close())
		req := httptest.NewRequest(method, path, &body)
		req.Header.Set("Test-User", strconv.Itoa(userID))
		req.Header.Set("New-Api-User", strconv.Itoa(userID))
		req.Header.Set("Content-Type", writer.FormDataContentType())
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	require.Equal(t, http.StatusOK, request("POST", "/api/ticket/", 1).Code)
	require.Equal(t, http.StatusOK, request("POST", "/api/ticket/1/messages", 1).Code)
	require.Equal(t, http.StatusTooManyRequests, request("POST", "/api/ticket/1/messages", 1).Code)
	require.Equal(t, http.StatusOK, request("POST", "/api/ticket/", 2).Code)
	// Login has its own quota even after the same IP exhausts its ticket quota.
	require.NotEqual(t, http.StatusTooManyRequests, request("POST", "/api/user/login", 0).Code)
	require.Equal(t, http.StatusTooManyRequests, request("POST", "/api/user/login", 0).Code)

	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	for i := 0; i < 50; i++ {
		message := &model.TicketMessage{Content: "screenshot"}
		for j := 0; j < 4; j++ {
			message.Attachments = append(message.Attachments, model.TicketAttachment{
				MimeType: "image/png", Data: buffer.Bytes(), Size: buffer.Len(),
			})
		}
		_, err := model.ReplyTicket(1, 1, false, message)
		require.NoError(t, err)
	}
	response := request("GET", "/api/ticket/1/messages", 1)
	require.Equal(t, http.StatusOK, response.Code)
	var page struct {
		Data struct {
			Items []model.TicketMessage `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 50)
	for _, message := range page.Data.Items {
		require.Len(t, message.Attachments, 4)
		for _, attachment := range message.Attachments {
			response := request("GET", fmt.Sprintf("/api/ticket/1/attachments/%d", attachment.Id), 1)
			require.Equal(t, http.StatusOK, response.Code)
			require.Equal(t, buffer.Bytes(), response.Body.Bytes())
		}
	}
	for i := 200; i < 240; i++ {
		require.Equal(t, http.StatusOK, request("GET", "/api/ticket/1/attachments/1", 1).Code)
	}
	require.Equal(t, http.StatusTooManyRequests, request("GET", "/api/ticket/1/attachments/1", 1).Code)
	require.Equal(t, http.StatusOK, request("GET", "/api/ticket/", 1).Code)
	require.Equal(t, http.StatusNotFound, request("GET", "/api/ticket/1/attachments/1", 2).Code)
	require.Equal(t, http.StatusUnauthorized, request("GET", "/api/ticket/1/attachments/1", 0).Code)
	require.Equal(t, http.StatusOK, request("GET", "/api/ticket/unread", 1).Code)
	require.Equal(t, http.StatusUnauthorized, request("GET", "/api/ticket/unread", 0).Code)
	readRequest := httptest.NewRequest("POST", "/api/ticket/1/read", bytes.NewBufferString(`{"message_id":1}`))
	readRequest.Header.Set("Test-User", "1")
	readRequest.Header.Set("New-Api-User", "1")
	readRequest.Header.Set("Content-Type", "application/json")
	readResponse := httptest.NewRecorder()
	engine.ServeHTTP(readResponse, readRequest)
	require.Equal(t, http.StatusOK, readResponse.Code)
}
