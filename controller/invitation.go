package controller

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type createInvitationRequest struct {
	Code   string `json:"code"`
	Remark string `json:"remark"`
}

func GetAllInvitations(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	invitations, total, err := model.GetInvitations(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(invitations)
	common.ApiSuccess(c, pageInfo)
}

func AddInvitation(c *gin.Context) {
	var request createInvitationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	request.Code = strings.TrimSpace(request.Code)
	request.Remark = strings.TrimSpace(request.Remark)
	if request.Code == "" {
		common.ApiErrorI18n(c, i18n.MsgInvitationCodeRequired)
		return
	}
	if err := common.Validate.Var(request.Code, "numeric,min=5,max=20"); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvitationCodeInvalidQQ)
		return
	}
	if request.Remark == "" {
		common.ApiErrorI18n(c, i18n.MsgInvitationRemarkRequired)
		return
	}
	if utf8.RuneCountInString(request.Remark) > 255 {
		common.ApiErrorI18n(c, i18n.MsgInvitationRemarkTooLong)
		return
	}

	exists, err := model.CheckUserExistOrDeleted(request.Code, "")
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if exists {
		common.ApiErrorI18n(c, i18n.MsgInvitationCodeExists)
		return
	}
	qqExists, err := model.QQIdExists(request.Code)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if qqExists {
		common.ApiErrorI18n(c, i18n.MsgInvitationCodeExists)
		return
	}
	invitationExists, err := model.InvitationCodeExists(request.Code)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if invitationExists {
		common.ApiErrorI18n(c, i18n.MsgInvitationCodeExists)
		return
	}

	invitation := model.Invitation{
		Code:        request.Code,
		Remark:      request.Remark,
		Status:      model.InvitationStatusAvailable,
		CreatedBy:   c.GetInt("id"),
		CreatedTime: common.GetTimestamp(),
	}
	if err := invitation.Insert(); err != nil {
		if exists, checkErr := model.InvitationCodeExists(request.Code); checkErr == nil && exists {
			common.ApiErrorI18n(c, i18n.MsgInvitationCodeExists)
			return
		}
		common.SysLog("failed to insert invitation: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgCreateFailed)
		return
	}
	common.ApiSuccess(c, invitation)
}

func DeleteInvitation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}
	if err := model.DeleteInvitationById(id); err != nil {
		switch {
		case errors.Is(err, model.ErrInvitationNotFound):
			common.ApiErrorI18n(c, i18n.MsgInvitationInvalid)
		case errors.Is(err, model.ErrInvitationUsed):
			common.ApiErrorI18n(c, i18n.MsgInvitationDeleteUsed)
		default:
			common.ApiErrorI18n(c, i18n.MsgDeleteFailed)
		}
		return
	}
	common.ApiSuccess(c, nil)
}
