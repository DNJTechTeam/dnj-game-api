package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/emailsignupcode/entities"
)

type EmailSignupCodeRepositoryInterface interface {
	FindByEmailForUpdate(ctx context.Context, email string) (*entities.EmailSignupCode, error)
	Create(ctx context.Context, code *entities.EmailSignupCode) (*entities.EmailSignupCode, error)
	Update(ctx context.Context, code *entities.EmailSignupCode) (*entities.EmailSignupCode, error)
}
