package interfaces

import (
	"context"
)

type EmailServiceInterface interface {
	SendVerificationCodeEmail(ctx context.Context, email string, verificationCode string) error
}
