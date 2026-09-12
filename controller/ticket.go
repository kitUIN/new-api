package controller

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ticketError(c *gin.Context, err error) {
	status, message := http.StatusInternalServerError, i18n.MsgTicketOperationFailed
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, message = http.StatusNotFound, i18n.MsgTicketNotFound
	case errors.Is(err, model.ErrTicketClosed):
		status, message = http.StatusConflict, i18n.MsgTicketClosed
	case errors.Is(err, service.ErrTicketInvalid):
		status, message = http.StatusBadRequest, err.Error()
	default:
		common.SysLog("support ticket error: " + err.Error())
	}
	ticketFailure(c, status, message)
}

func ticketFailure(c *gin.Context, status int, key string) {
	c.JSON(status, gin.H{"success": false, "message": common.TranslateMessage(c, key)})
}

func ticketParam(c *gin.Context, name string) (int, bool) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil || id <= 0 {
		ticketFailure(c, http.StatusBadRequest, i18n.MsgInvalidParams)
		return 0, false
	}
	return id, true
}

func readTicketInput(c *gin.Context) (string, string, [][]byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.TicketMaxRequestBytes)
	err := c.Request.ParseMultipartForm(service.TicketMaxRequestBytes)
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		ticketFailure(c, http.StatusBadRequest, i18n.MsgTicketInvalidUpload)
		return "", "", nil, false
	}
	files := c.Request.MultipartForm.File["images"]
	if len(files) > service.TicketMaxImages {
		ticketFailure(c, http.StatusBadRequest, i18n.MsgTicketTooManyImages)
		return "", "", nil, false
	}
	images := make([][]byte, 0, len(files))
	for _, file := range files {
		if file.Size > service.TicketMaxImageBytes {
			ticketFailure(c, http.StatusBadRequest, i18n.MsgTicketImageTooLarge)
			return "", "", nil, false
		}
		reader, err := file.Open()
		if err != nil {
			ticketError(c, err)
			return "", "", nil, false
		}
		data, err := io.ReadAll(io.LimitReader(reader, service.TicketMaxImageBytes+1))
		_ = reader.Close()
		if err != nil {
			ticketError(c, err)
			return "", "", nil, false
		}
		images = append(images, data)
	}
	return c.Request.PostForm.Get("title"), c.Request.PostForm.Get("content"), images, true
}

func GetTickets(c *gin.Context) {
	status := c.Query("status")
	if status != "" && status != model.TicketStatusOpen && status != model.TicketStatusClosed {
		ticketFailure(c, http.StatusBadRequest, i18n.MsgInvalidParams)
		return
	}
	page := common.GetPageQuery(c)
	if page.Page < 1 || page.PageSize < 1 {
		ticketFailure(c, http.StatusBadRequest, i18n.MsgInvalidParams)
		return
	}
	tickets, total, err := model.ListTickets(c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser, status, page.GetStartIdx(), page.GetPageSize())
	if err != nil {
		ticketError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(tickets)
	common.ApiSuccess(c, page)
}

func AddTicket(c *gin.Context) {
	title, content, images, ok := readTicketInput(c)
	if !ok {
		return
	}
	ticket, err := service.CreateSupportTicket(c.GetInt("id"), c.GetString("username"), title, content, images)
	if err != nil {
		ticketError(c, err)
		return
	}
	common.ApiSuccess(c, ticket)
}

func GetTicket(c *gin.Context) {
	id, ok := ticketParam(c, "id")
	if !ok {
		return
	}
	ticket, err := model.GetTicket(id, c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser)
	if err != nil {
		ticketError(c, err)
		return
	}
	common.ApiSuccess(c, ticket)
}

func GetTicketMessages(c *gin.Context) {
	id, ok := ticketParam(c, "id")
	if !ok {
		return
	}
	before, err := strconv.Atoi(c.DefaultQuery("before", "0"))
	if err != nil || before < 0 {
		ticketFailure(c, http.StatusBadRequest, i18n.MsgInvalidParams)
		return
	}
	messages, hasMore, err := model.GetTicketMessages(id, c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser, before)
	if err != nil {
		ticketError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": messages, "has_more": hasMore})
}

func ReplyTicket(c *gin.Context) {
	id, ok := ticketParam(c, "id")
	if !ok {
		return
	}
	admin := c.GetInt("role") >= common.RoleAdminUser
	ticket, err := model.GetTicket(id, c.GetInt("id"), admin)
	if err != nil {
		ticketError(c, err)
		return
	}
	if ticket.Status == model.TicketStatusClosed {
		ticketError(c, model.ErrTicketClosed)
		return
	}
	_, content, images, ok := readTicketInput(c)
	if !ok {
		return
	}
	ticket, err = service.ReplySupportTicket(id, c.GetInt("id"), c.GetString("username"), admin, content, images)
	if err != nil {
		ticketError(c, err)
		return
	}
	common.ApiSuccess(c, ticket)
}

func CloseTicket(c *gin.Context) {
	id, ok := ticketParam(c, "id")
	if !ok {
		return
	}
	ticket, err := model.CloseTicket(id, c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser)
	if err != nil {
		ticketError(c, err)
		return
	}
	common.ApiSuccess(c, ticket)
}

func GetTicketImage(c *gin.Context) {
	id, ok := ticketParam(c, "id")
	if !ok {
		return
	}
	attachmentId, ok := ticketParam(c, "attachment_id")
	if !ok {
		return
	}
	attachment, err := model.GetTicketAttachment(id, attachmentId, c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser)
	if err != nil {
		ticketError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Data(http.StatusOK, attachment.MimeType, attachment.Data)
}
