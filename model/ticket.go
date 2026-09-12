package model

import (
	"errors"
	"slices"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TicketStatusOpen      = "open"
	TicketStatusClosed    = "closed"
	TicketMessagePageSize = 50
)

var ErrTicketClosed = errors.New("ticket is closed")

type Ticket struct {
	Id               int    `json:"id"`
	UserId           int    `json:"user_id" gorm:"index:idx_ticket_user_updated,priority:1"`
	Username         string `json:"username" gorm:"size:64"`
	Title            string `json:"title" gorm:"size:120"`
	Status           string `json:"status" gorm:"size:16;index:idx_ticket_status_updated,priority:1"`
	LastReplyByAdmin bool   `json:"last_reply_by_admin"`
	MessageCount     int    `json:"message_count"`
	CreatedTime      int64  `json:"created_time"`
	UpdatedTime      int64  `json:"updated_time" gorm:"index:idx_ticket_user_updated,priority:2;index:idx_ticket_status_updated,priority:2;index"`
	ClosedTime       int64  `json:"closed_time"`
	ClosedBy         int    `json:"closed_by"`
}

type TicketMessage struct {
	Id          int                `json:"id"`
	TicketId    int                `json:"ticket_id" gorm:"index:idx_ticket_message,priority:1"`
	UserId      int                `json:"user_id"`
	Username    string             `json:"username" gorm:"size:64"`
	IsAdmin     bool               `json:"is_admin"`
	Content     string             `json:"content" gorm:"type:text"`
	CreatedTime int64              `json:"created_time"`
	Attachments []TicketAttachment `json:"attachments" gorm:"-"`
}

type TicketAttachment struct {
	Id        int    `json:"id"`
	TicketId  int    `json:"ticket_id" gorm:"index"`
	MessageId int    `json:"message_id" gorm:"index"`
	MimeType  string `json:"mime_type" gorm:"size:32"`
	Size      int    `json:"size"`
	Data      []byte `json:"-"`
}

func ticketScope(db *gorm.DB, userId int, admin bool) *gorm.DB {
	db = db.Model(&Ticket{})
	if !admin {
		db = db.Where("user_id = ?", userId)
	}
	return db
}

func GetTicket(id, userId int, admin bool) (*Ticket, error) {
	ticket := &Ticket{}
	err := ticketScope(DB, userId, admin).Where("id = ?", id).First(ticket).Error
	return ticket, err
}

func ListTickets(userId int, admin bool, status string, offset, limit int) ([]Ticket, int64, error) {
	query := ticketScope(DB, userId, admin)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	tickets := make([]Ticket, 0)
	err := query.Order("updated_time DESC, id DESC").Offset(offset).Limit(limit).Find(&tickets).Error
	return tickets, total, err
}

func insertTicketMessage(tx *gorm.DB, message *TicketMessage) error {
	if err := tx.Create(message).Error; err != nil {
		return err
	}
	for i := range message.Attachments {
		attachment := &message.Attachments[i]
		attachment.TicketId = message.TicketId
		attachment.MessageId = message.Id
		if err := tx.Create(attachment).Error; err != nil {
			return err
		}
	}
	return nil
}

func CreateTicket(ticket *Ticket, message *TicketMessage) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		ticket.Status = TicketStatusOpen
		ticket.CreatedTime = common.GetTimestamp()
		ticket.UpdatedTime = ticket.CreatedTime
		ticket.MessageCount = 1
		ticket.LastReplyByAdmin = message.IsAdmin
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		message.TicketId = ticket.Id
		message.CreatedTime = ticket.CreatedTime
		return insertTicketMessage(tx, message)
	})
}

func ReplyTicket(id, userId int, admin bool, message *TicketMessage) (*Ticket, error) {
	ticket := &Ticket{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		// A conditional write locks the ticket on all supported databases, including SQLite.
		// CloseTicket updates the same row, so closing and replying cannot pass each other.
		result := ticketScope(tx, userId, admin).Where("id = ? AND status = ?", id, TicketStatusOpen).
			Updates(map[string]interface{}{
				"updated_time": common.GetTimestamp(), "last_reply_by_admin": admin,
				"message_count": gorm.Expr("message_count + ?", 1),
			})
		if result.Error != nil {
			return result.Error
		}
		if err := ticketScope(tx, userId, admin).Where("id = ?", id).First(ticket).Error; err != nil {
			return err
		}
		if result.RowsAffected == 0 {
			return ErrTicketClosed
		}
		message.TicketId = id
		message.UserId = userId
		message.IsAdmin = admin
		message.CreatedTime = ticket.UpdatedTime
		return insertTicketMessage(tx, message)
	})
	return ticket, err
}

func CloseTicket(id, userId int, admin bool) (*Ticket, error) {
	ticket := &Ticket{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		now := common.GetTimestamp()
		result := ticketScope(tx, userId, admin).Where("id = ? AND status = ?", id, TicketStatusOpen).
			Updates(map[string]interface{}{
				"status": TicketStatusClosed, "closed_by": userId,
				"closed_time": now, "updated_time": now,
			})
		if result.Error != nil {
			return result.Error
		}
		return ticketScope(tx, userId, admin).Where("id = ?", id).First(ticket).Error
	})
	return ticket, err
}

func GetTicketMessages(id, userId int, admin bool, before int) ([]TicketMessage, bool, error) {
	if _, err := GetTicket(id, userId, admin); err != nil {
		return nil, false, err
	}
	query := DB.Where("ticket_id = ?", id)
	if before > 0 {
		query = query.Where("id < ?", before)
	}
	messages := make([]TicketMessage, 0)
	if err := query.Order("id DESC").Limit(TicketMessagePageSize + 1).Find(&messages).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(messages) > TicketMessagePageSize
	if hasMore {
		messages = messages[:TicketMessagePageSize]
	}
	slices.Reverse(messages)
	ids := make([]int, 0, len(messages))
	for i := range messages {
		ids = append(ids, messages[i].Id)
		messages[i].Attachments = make([]TicketAttachment, 0)
	}
	if len(ids) > 0 {
		var attachments []TicketAttachment
		if err := DB.Omit("data").Where("ticket_id = ? AND message_id IN ?", id, ids).Order("id ASC").Find(&attachments).Error; err != nil {
			return nil, false, err
		}
		byMessage := make(map[int][]TicketAttachment)
		for _, attachment := range attachments {
			byMessage[attachment.MessageId] = append(byMessage[attachment.MessageId], attachment)
		}
		for i := range messages {
			if items := byMessage[messages[i].Id]; len(items) > 0 {
				messages[i].Attachments = items
			}
		}
	}
	return messages, hasMore, nil
}

func GetTicketAttachment(id, attachmentId, userId int, admin bool) (*TicketAttachment, error) {
	if _, err := GetTicket(id, userId, admin); err != nil {
		return nil, err
	}
	attachment := &TicketAttachment{}
	err := DB.Where("id = ? AND ticket_id = ?", attachmentId, id).First(attachment).Error
	return attachment, err
}
