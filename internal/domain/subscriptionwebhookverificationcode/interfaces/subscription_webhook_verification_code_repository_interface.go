package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/entities"
)

type SubscriptionWebhookVerificationCodeRepositoryInterface interface {
	Create(ctx context.Context, code *entities.SubscriptionWebhookVerificationCode) (*entities.SubscriptionWebhookVerificationCode, error)
	// FindByEmail returns nil, nil when no record exists for that email.
	FindByEmail(ctx context.Context, email string) (*entities.SubscriptionWebhookVerificationCode, error)
	Update(ctx context.Context, code *entities.SubscriptionWebhookVerificationCode) (*entities.SubscriptionWebhookVerificationCode, error)
	// FindByEmailAndCode returns nil, nil when no record matches both fields.
	FindByEmailAndCode(ctx context.Context, email string, verificationCode string) (*entities.SubscriptionWebhookVerificationCode, error)
}
