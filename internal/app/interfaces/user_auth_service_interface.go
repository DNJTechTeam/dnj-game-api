package interfaces

import (
	"context"

	"github.com/scienceandcode/nucleus-api/internal/app/messages"
)

type UserAuthServiceInterface interface {
	Register(ctx context.Context, request *messages.RegisterRequestDTO) (*messages.CreateUserResponseDTO, error)
	Login(ctx context.Context, request *messages.LoginRequestDTO) (*messages.LoginResponseDTO, string, error)
	ConfirmEmail(ctx context.Context, token string) (*messages.ConfirmEmailResponseDTO, error)
	ResendConfirmationEmail(ctx context.Context) (*messages.ResendConfirmationEmailResponseDTO, error)
	UpdatePassword(ctx context.Context, request *messages.UpdatePasswordRequestDTO) (*messages.UpdatePasswordResponseDTO, error)
	ForgotPassword(ctx context.Context, request *messages.ForgotPasswordRequestDTO) error
}
