package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestTicketUnreadMessagesAndReadCursors(t *testing.T) {
	_, router := setupTicketTest(t)
	unread := func(userId int) int {
		t.Helper()
		return ticketData[struct {
			UnreadCount int `json:"unread_count"`
		}](t, ticketRequest(t, router, userId, "GET", "/ticket/unread", "", "")).UnreadCount
	}
	markRead := func(userId, ticketId int, body string) int {
		t.Helper()
		req := httptest.NewRequest("POST", fmt.Sprintf("/ticket/%d/read", ticketId), strings.NewReader(body))
		req.Header.Set("Test-User", strconv.Itoa(userId))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response.Code
	}
	body := func(messageId int) string { return fmt.Sprintf(`{"message_id":%d}`, messageId) }
	first := &model.Ticket{UserId: 1, Title: "first"}
	firstMessage := &model.TicketMessage{UserId: 1, Content: "help"}
	require.NoError(t, model.CreateTicket(first, firstMessage))
	second := &model.Ticket{UserId: 2, Title: "second"}
	secondMessage := &model.TicketMessage{UserId: 2, Content: "other user"}
	require.NoError(t, model.CreateTicket(second, secondMessage))
	require.Zero(t, unread(1))
	require.Zero(t, unread(2))
	require.Equal(t, 2, unread(99))
	require.Equal(t, 2, unread(100))

	reply := func() *model.TicketMessage {
		t.Helper()
		message := &model.TicketMessage{Content: "admin reply"}
		_, err := model.ReplyTicket(first.Id, 99, true, message)
		require.NoError(t, err)
		return message
	}
	firstReply, secondReply := reply(), reply()
	require.Equal(t, 2, unread(1)) // Count messages, not tickets.
	require.Zero(t, unread(2))
	require.Equal(t, 2, unread(99)) // Own messages never become unread.
	require.Equal(t, 4, unread(100))

	// Fetching a list/page (including background polling) is not a read receipt.
	require.Equal(t, http.StatusOK, ticketRequest(t, router, 1, "GET", "/ticket/", "", "").Code)
	require.Equal(t, http.StatusOK, ticketRequest(t, router, 1, "GET", fmt.Sprintf("/ticket/%d/messages", first.Id), "", "").Code)
	require.Equal(t, 2, unread(1))

	// A reply arriving after the displayed page must stay unread.
	latestReply := reply()
	require.Equal(t, http.StatusOK, markRead(1, first.Id, body(secondReply.Id)))
	require.Equal(t, 1, unread(1))
	// Duplicate or delayed receipts from another tab cannot undo progress.
	require.Equal(t, http.StatusOK, markRead(1, first.Id, body(firstReply.Id)))
	require.Equal(t, http.StatusOK, markRead(1, first.Id, body(secondReply.Id)))
	require.Equal(t, 1, unread(1))
	var cursor model.TicketRead
	require.NoError(t, model.DB.Where("user_id = ? AND ticket_id = ?", 1, first.Id).First(&cursor).Error)
	require.Equal(t, secondReply.Id, cursor.LastReadMessageId)

	// Closing is not reading, and messages on closed tickets can still be read.
	_, err := model.CloseTicket(first.Id, 99, true)
	require.NoError(t, err)
	require.Equal(t, 1, unread(1))
	require.Equal(t, http.StatusOK, markRead(1, first.Id, body(latestReply.Id)))
	require.Zero(t, unread(1))

	// Each admin has an independent cursor across all visible tickets.
	require.Equal(t, http.StatusOK, markRead(99, first.Id, body(latestReply.Id)))
	require.Equal(t, 1, unread(99))
	require.Equal(t, 5, unread(100))
	require.Equal(t, http.StatusOK, markRead(99, second.Id, body(secondMessage.Id)))
	require.Zero(t, unread(99))
	require.Equal(t, 5, unread(100))

	for _, invalid := range []string{`{}`, `{"message_id":0}`, `{"message_id":-1}`, `{"message_id":"bad"}`, `not json`} {
		require.Equal(t, http.StatusBadRequest, markRead(1, first.Id, invalid))
	}
	require.Equal(t, http.StatusNotFound, markRead(1, first.Id, body(secondMessage.Id)))
	require.Equal(t, http.StatusNotFound, markRead(1, first.Id, body(999999)))
	require.Equal(t, http.StatusNotFound, markRead(1, second.Id, body(secondMessage.Id)))
	require.Equal(t, http.StatusUnauthorized, markRead(0, first.Id, body(firstMessage.Id)))
	require.Equal(t, http.StatusUnauthorized, ticketRequest(t, router, 0, "GET", "/ticket/unread", "", "").Code)
	require.Zero(t, unread(1))
	require.Zero(t, unread(2))
}
