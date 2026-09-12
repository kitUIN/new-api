package controller

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTicketTest(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.Ticket{}, &model.TicketMessage{}, &model.TicketAttachment{}, &model.TicketRead{}))
	router := gin.New()
	// Controller tests provide the identity normally populated by UserAuth.
	router.Use(func(c *gin.Context) {
		id, _ := strconv.Atoi(c.GetHeader("Test-User"))
		if id == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set("id", id)
		c.Set("username", fmt.Sprintf("user-%d", id))
		role := common.RoleCommonUser
		if id == 99 || id == 100 {
			role = common.RoleAdminUser
		}
		c.Set("role", role)
	})
	router.GET("/ticket/", GetTickets)
	router.GET("/ticket/unread", GetUnreadTicketCount)
	router.POST("/ticket/", AddTicket)
	router.GET("/ticket/:id", GetTicket)
	router.GET("/ticket/:id/messages", GetTicketMessages)
	router.POST("/ticket/:id/messages", ReplyTicket)
	router.POST("/ticket/:id/close", CloseTicket)
	router.POST("/ticket/:id/read", MarkTicketRead)
	router.GET("/ticket/:id/attachments/:attachment_id", GetTicketImage)
	return db, router
}

func ticketRequest(t *testing.T, router *gin.Engine, user int, method, path, title, content string, images ...[]byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("title", title))
	require.NoError(t, writer.WriteField("content", content))
	for _, data := range images {
		part, err := writer.CreateFormFile("images", "screenshot.png")
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Test-User", strconv.Itoa(user))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func ticketData[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var envelope struct {
		Success bool `json:"success"`
		Data    T    `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	return envelope.Data
}

func ticketPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	return buffer.Bytes()
}

func TestTicketConversationPermissionsAndQQNotifications(t *testing.T) {
	_, router := setupTicketTest(t)
	images := ticketPNG(t)
	var oldAddress, oldToken, oldAdmin = common.QQCallbackAddress, common.QQCallbackAccessToken, common.QQAdminNumber
	t.Cleanup(func() {
		common.QQCallbackAddress, common.QQCallbackAccessToken, common.QQAdminNumber = oldAddress, oldToken, oldAdmin
	})
	type notification struct {
		path, authorization, accessToken string
		payload                          map[string]string
	}
	notifications := make(chan notification, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := make(map[string]string)
		_ = common.DecodeJson(r.Body, &payload)
		notifications <- notification{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-Access-Token"), payload}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	common.QQCallbackAddress, common.QQCallbackAccessToken, common.QQAdminNumber = server.URL, "test-secret", "123456"
	waitNotification := func(event string, id int) {
		t.Helper()
		select {
		case notification := <-notifications:
			require.Equal(t, "/api/nachoai/send_message", notification.path)
			require.Equal(t, "test-secret", notification.authorization)
			require.Equal(t, "test-secret", notification.accessToken)
			require.Equal(t, "123456", notification.payload["qq"])
			require.Equal(t, "123456", notification.payload["admin_qq"])
			require.Equal(t, "123456", notification.payload["to"])
			require.Equal(t, notification.payload["message"], notification.payload["content"])
			require.Contains(t, notification.payload["message"], event)
			require.Contains(t, notification.payload["message"], fmt.Sprintf("/tickets?ticket=%d", id))
		case <-time.After(3 * time.Second):
			t.Fatal("missing QQ notification")
		}
	}
	created := ticketData[model.Ticket](t, ticketRequest(t, router, 1, "POST", "/ticket/", "API error", "first message", images))
	waitNotification("新建工单", created.Id)
	path := fmt.Sprintf("/ticket/%d", created.Id)
	for _, route := range []string{path, path + "/messages", path + "/attachments/1"} {
		require.Equal(t, 404, ticketRequest(t, router, 2, "GET", route, "", "").Code)
		require.Equal(t, 401, ticketRequest(t, router, 0, "GET", route, "", "").Code)
	}
	for _, route := range []string{path + "/messages", path + "/close"} {
		require.Equal(t, 404, ticketRequest(t, router, 2, "POST", route, "", "intruder").Code)
	}
	type ticketList struct {
		Total int            `json:"total"`
		Items []model.Ticket `json:"items"`
	}
	require.Zero(t, ticketData[ticketList](t, ticketRequest(t, router, 2, "GET", "/ticket/", "", "")).Total)
	require.Equal(t, 1, ticketData[ticketList](t, ticketRequest(t, router, 99, "GET", "/ticket/", "", "")).Total)
	adminReply := ticketData[model.Ticket](t, ticketRequest(t, router, 99, "POST", path+"/messages", "", "admin reply", images))
	require.True(t, adminReply.LastReplyByAdmin)
	followup := ticketData[model.Ticket](t, ticketRequest(t, router, 1, "POST", path+"/messages", "", "", images))
	require.False(t, followup.LastReplyByAdmin)
	waitNotification("用户追问", created.Id)
	var page = ticketData[struct {
		Items []model.TicketMessage `json:"items"`
	}](t, ticketRequest(t, router, 1, "GET", path+"/messages", "", ""))
	require.Len(t, page.Items, 3)
	require.True(t, page.Items[1].IsAdmin)
	require.False(t, page.Items[2].IsAdmin)
	require.Equal(t, "first message", page.Items[0].Content)
	require.Empty(t, page.Items[0].Attachments[0].Data)
	for _, user := range []int{1, 99} {
		response := ticketRequest(t, router, user, "GET", path+"/attachments/1", "", "")
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, images, response.Body.Bytes())
		require.Equal(t, "image/png", response.Header().Get("Content-Type"))
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	}
	closed := ticketData[model.Ticket](t, ticketRequest(t, router, 1, "POST", path+"/close", "", ""))
	require.Equal(t, "closed", closed.Status)
	require.Equal(t, 1, closed.ClosedBy)
	for _, user := range []int{1, 99} {
		require.Equal(t, http.StatusConflict, ticketRequest(t, router, user, "POST", path+"/messages", "", "too late", images).Code)
		reclosed := ticketData[model.Ticket](t, ticketRequest(t, router, user, "POST", path+"/close", "", ""))
		require.Equal(t, closed.ClosedBy, reclosed.ClosedBy)
		require.Equal(t, 3, reclosed.MessageCount)
	}
	require.Zero(t, ticketData[ticketList](t, ticketRequest(t, router, 99, "GET", "/ticket/?status=open", "", "")).Total)
	require.Equal(t, 1, ticketData[ticketList](t, ticketRequest(t, router, 99, "GET", "/ticket/?status=closed", "", "")).Total)
	select {
	case unexpected := <-notifications:
		t.Fatalf("unexpected notification: %+v", unexpected)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTicketValidationAndAdminClose(t *testing.T) {
	_, router := setupTicketTest(t)
	oldAddress := common.QQCallbackAddress
	common.QQCallbackAddress = ""
	t.Cleanup(func() { common.QQCallbackAddress = oldAddress })
	valid := ticketPNG(t)
	for _, tc := range []struct {
		title, content string
		images         [][]byte
	}{
		{"", "text", nil}, {strings.Repeat("a", 121), "text", nil}, {"title", "  ", nil},
		{"title", strings.Repeat("字", 10001), nil}, {"title", "text", [][]byte{[]byte("<svg></svg>")}},
		{"title", "text", [][]byte{bytes.Repeat([]byte("a"), service.TicketMaxImageBytes+1)}},
		{"title", "text", [][]byte{valid, valid, valid, valid, valid}},
	} {
		require.Equal(t, 400, ticketRequest(t, router, 1, "POST", "/ticket/", tc.title, tc.content, tc.images...).Code)
	}
	for _, query := range []string{"page_size=-1", "p=-1", "status=invalid"} {
		require.Equal(t, 400, ticketRequest(t, router, 1, "GET", "/ticket/?"+query, "", "").Code)
	}
	created := ticketData[model.Ticket](t, ticketRequest(t, router, 1, "POST", "/ticket/", "title", "text"))
	path := fmt.Sprintf("/ticket/%d", created.Id)
	require.Equal(t, 404, ticketRequest(t, router, 1, "GET", path+"/attachments/999", "", "").Code)
	closed := ticketData[model.Ticket](t, ticketRequest(t, router, 99, "POST", path+"/close", "", ""))
	require.Equal(t, 99, closed.ClosedBy)
	for _, user := range []int{1, 99} {
		require.Equal(t, 409, ticketRequest(t, router, user, "POST", path+"/messages", "", "text").Code)
	}
}

func TestTicketRejectsTruncatedImagesWithoutSavingMessages(t *testing.T) {
	db, router := setupTicketTest(t)
	oldAddress := common.QQCallbackAddress
	common.QQCallbackAddress = ""
	t.Cleanup(func() { common.QQCallbackAddress = oldAddress })
	truncated := ticketPNG(t)[:33] // Complete PNG header, missing pixel data.
	_, _, err := image.DecodeConfig(bytes.NewReader(truncated))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, ticketRequest(t, router, 1, "POST", "/ticket/", "title", "", truncated).Code)
	var count int64
	require.NoError(t, db.Model(&model.Ticket{}).Count(&count).Error)
	require.Zero(t, count)
	created := ticketData[model.Ticket](t, ticketRequest(t, router, 1, "POST", "/ticket/", "title", "first"))
	path := fmt.Sprintf("/ticket/%d/messages", created.Id)
	require.Equal(t, http.StatusBadRequest, ticketRequest(t, router, 1, "POST", path, "", "", truncated).Code)
	stored, err := model.GetTicket(created.Id, 1, false)
	require.NoError(t, err)
	require.Equal(t, 1, stored.MessageCount)
	require.NoError(t, db.Model(&model.TicketAttachment{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestTicketMessagePaginationAndRollback(t *testing.T) {
	db, _ := setupTicketTest(t)
	ticket := &model.Ticket{UserId: 1, Username: "owner", Title: "history"}
	require.NoError(t, model.CreateTicket(ticket, &model.TicketMessage{UserId: 1, Content: "first"}))
	for i := 0; i < 55; i++ {
		_, err := model.ReplyTicket(ticket.Id, 1, false, &model.TicketMessage{Content: strconv.Itoa(i)})
		require.NoError(t, err)
	}
	latest, more, err := model.GetTicketMessages(ticket.Id, 1, false, 0)
	require.NoError(t, err)
	require.True(t, more)
	require.Len(t, latest, 50)
	earlier, more, err := model.GetTicketMessages(ticket.Id, 1, false, latest[0].Id)
	require.NoError(t, err)
	require.False(t, more)
	require.Len(t, earlier, 6)
	require.Less(t, earlier[5].Id, latest[0].Id)
	require.Equal(t, "first", earlier[0].Content)
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("ticket_attachment_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "ticket_attachments" {
			tx.AddError(io.ErrUnexpectedEOF)
		}
	}))
	_, err = model.ReplyTicket(ticket.Id, 99, true, &model.TicketMessage{Content: "rollback", Attachments: []model.TicketAttachment{{Data: ticketPNG(t)}}})
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	saved, err := model.GetTicket(ticket.Id, 1, false)
	require.NoError(t, err)
	require.Equal(t, 56, saved.MessageCount)
	require.False(t, saved.LastReplyByAdmin)
	var messageCount int64
	require.NoError(t, db.Model(&model.TicketMessage{}).Where("ticket_id = ?", ticket.Id).Count(&messageCount).Error)
	require.EqualValues(t, 56, messageCount)
	_, err = model.CloseTicket(ticket.Id, 99, true)
	require.NoError(t, err)
	_, err = model.ReplyTicket(ticket.Id, 1, false, &model.TicketMessage{Content: "closed"})
	require.True(t, errors.Is(err, model.ErrTicketClosed))
}

func TestTicketConcurrentCloseAndReply(t *testing.T) {
	_, _ = setupTicketTest(t)
	for attempt := 0; attempt < 20; attempt++ {
		ticket := &model.Ticket{UserId: 1, Title: "concurrent close"}
		require.NoError(t, model.CreateTicket(ticket, &model.TicketMessage{UserId: 1, Content: "first"}))
		start := make(chan struct{})
		replyResult, closeResult := make(chan error, 1), make(chan error, 1)
		go func() {
			<-start
			_, err := model.ReplyTicket(ticket.Id, 1, false, &model.TicketMessage{Content: "reply"})
			replyResult <- err
		}()
		go func() {
			<-start
			_, err := model.CloseTicket(ticket.Id, 99, true)
			closeResult <- err
		}()
		close(start)
		replyErr, closeErr := <-replyResult, <-closeResult
		require.NoError(t, closeErr)
		if replyErr != nil {
			require.ErrorIs(t, replyErr, model.ErrTicketClosed)
		}
		saved, err := model.GetTicket(ticket.Id, 1, false)
		require.NoError(t, err)
		require.Equal(t, model.TicketStatusClosed, saved.Status)
		expectedCount := 1
		if replyErr == nil {
			expectedCount++
		}
		require.Equal(t, expectedCount, saved.MessageCount)
		_, err = model.ReplyTicket(ticket.Id, 99, true, &model.TicketMessage{Content: "after close"})
		require.ErrorIs(t, err, model.ErrTicketClosed)
	}
}
