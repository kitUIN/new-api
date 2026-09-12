package service

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	_ "golang.org/x/image/webp"
)

const (
	TicketMaxImages       = 4
	TicketMaxImageBytes   = 2 << 20
	TicketMaxRequestBytes = TicketMaxImages*TicketMaxImageBytes + (128 << 10)
)

var ErrTicketInvalid = errors.New("invalid ticket input")

type ticketInputError string

func (e ticketInputError) Error() string { return string(e) }
func (e ticketInputError) Unwrap() error { return ErrTicketInvalid }

func ValidateTicketInput(title, content string, images [][]byte, creating bool) (string, string, []model.TicketAttachment, error) {
	title, content = strings.TrimSpace(title), strings.TrimSpace(content)
	if creating && (title == "" || utf8.RuneCountInString(title) > 120 || !utf8.ValidString(title)) {
		return "", "", nil, ticketInputError(i18n.MsgTicketInvalidTitle)
	}
	if !utf8.ValidString(content) || utf8.RuneCountInString(content) > 10000 || (content == "" && len(images) == 0) {
		return "", "", nil, ticketInputError(i18n.MsgTicketInvalidContent)
	}
	if len(images) > TicketMaxImages {
		return "", "", nil, ticketInputError(i18n.MsgTicketTooManyImages)
	}
	attachments := make([]model.TicketAttachment, 0, len(images))
	for _, data := range images {
		if len(data) == 0 || len(data) > TicketMaxImageBytes {
			return "", "", nil, ticketInputError(i18n.MsgTicketImageTooLarge)
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		mimeType := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif", "webp": "image/webp"}[format]
		if err != nil || mimeType == "" || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 25_000_000 {
			return "", "", nil, ticketInputError(i18n.MsgTicketInvalidImage)
		}
		attachments = append(attachments, model.TicketAttachment{MimeType: mimeType, Size: len(data), Data: data})
	}
	return title, content, attachments, nil
}

func CreateSupportTicket(userId int, username, title, content string, images [][]byte) (*model.Ticket, error) {
	title, content, attachments, err := ValidateTicketInput(title, content, images, true)
	if err != nil {
		return nil, err
	}
	ticket := &model.Ticket{UserId: userId, Username: username, Title: title}
	message := &model.TicketMessage{UserId: userId, Username: username, Content: content, Attachments: attachments}
	if err := model.CreateTicket(ticket, message); err != nil {
		return nil, err
	}
	notifyTicketAdmin(ticket, message, "新建工单")
	return ticket, nil
}

func ReplySupportTicket(id, userId int, username string, admin bool, content string, images [][]byte) (*model.Ticket, error) {
	_, content, attachments, err := ValidateTicketInput("", content, images, false)
	if err != nil {
		return nil, err
	}
	message := &model.TicketMessage{Username: username, Content: content, Attachments: attachments}
	ticket, err := model.ReplyTicket(id, userId, admin, message)
	if err != nil {
		return nil, err
	}
	if !admin {
		notifyTicketAdmin(ticket, message, "用户追问")
	}
	return ticket, nil
}

func notifyTicketAdmin(ticket *model.Ticket, message *model.TicketMessage, event string) {
	preview := []rune(message.Content)
	if len(preview) > 500 {
		preview = append(preview[:500], []rune("...")...)
	}
	text := fmt.Sprintf("%s #%d\n标题：%s\n用户：%s (ID %d)\n%s\n图片：%d 张\n%s/tickets?ticket=%d",
		event, ticket.Id, ticket.Title, message.Username, message.UserId, string(preview), len(message.Attachments),
		strings.TrimRight(system_setting.ServerAddress, "/"), ticket.Id)
	common.SendQQAdminMessage(text, "support ticket notify")
}
