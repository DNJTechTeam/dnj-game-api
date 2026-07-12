package interfaces

import (
	"context"
)

type EmailServiceInterface interface {
	SendEmailConfirmation(ctx context.Context, email string, confirmationToken string) error
	SendPasswordRecoveryEmail(ctx context.Context, email string, temporaryPassword string) error
}
