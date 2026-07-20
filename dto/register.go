package dto

type RegisterRequest struct {
	InviteCode       string `json:"invite_code" validate:"required,numeric,min=5,max=20"`
	DisplayName      string `json:"display_name" validate:"required,max=20"`
	Password         string `json:"password" validate:"required,min=8,max=20"`
	Email            string `json:"email" validate:"omitempty,max=50,email"`
	VerificationCode string `json:"verification_code"`
}
