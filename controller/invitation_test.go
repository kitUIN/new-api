package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type invitationCreateResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Data    model.Invitation `json:"data"`
}

func addInvitationForTest(t *testing.T, payload map[string]any) invitationCreateResponse {
	t.Helper()
	body, err := common.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 100)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/invitation/", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	AddInvitation(ctx)

	var response invitationCreateResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return response
}

func TestAddInvitationAllowsEmptyRemarkAndRequiresInviter(t *testing.T) {
	db := setupInvitationRegisterControllerTestDB(t)
	inviter := model.User{
		Username:    "invitation-owner",
		Password:    "hashed-password",
		DisplayName: "Invitation Owner",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	if err := db.Create(&inviter).Error; err != nil {
		t.Fatalf("failed to seed inviter: %v", err)
	}

	response := addInvitationForTest(t, map[string]any{
		"code":       "77889900",
		"remark":     "   ",
		"inviter_id": inviter.Id,
	})
	if !response.Success {
		t.Fatalf("expected invitation creation to succeed, got %q", response.Message)
	}
	if response.Data.Remark != "" || response.Data.InviterId != inviter.Id {
		t.Fatalf("unexpected invitation response: %+v", response.Data)
	}

	var saved model.Invitation
	if err := db.First(&saved, response.Data.Id).Error; err != nil {
		t.Fatalf("failed to reload invitation: %v", err)
	}
	if saved.Remark != "" || saved.InviterId != inviter.Id {
		t.Fatalf("unexpected saved invitation: %+v", saved)
	}

	missingInviter := addInvitationForTest(t, map[string]any{"code": "88990011"})
	if missingInviter.Success {
		t.Fatal("expected missing inviter to be rejected")
	}

	invalidInviter := addInvitationForTest(t, map[string]any{
		"code":       "99001122",
		"inviter_id": inviter.Id + 1000,
	})
	if invalidInviter.Success {
		t.Fatal("expected unknown inviter to be rejected")
	}
}
