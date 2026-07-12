package interfaces

import (
	"context"
	"time"
)

type EmailConfirmationClaims struct {
	UserID    string
	Email     string
	ExpiresAt time.Time
	IssuedAt  time.Time
}

type EmailConfirmationServiceInterface interface {
	GenerateConfirmationToken(ctx context.Context, userID uint64, email string) (string, error)
	ValidateConfirmationToken(ctx context.Context, tokenString string) (*EmailConfirmationClaims, error)
}
