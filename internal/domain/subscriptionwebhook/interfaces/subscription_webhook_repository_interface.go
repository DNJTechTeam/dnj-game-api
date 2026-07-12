package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/entities"
)

type SubscriptionWebhookRepositoryInterface interface {
	Create(ctx context.Context, webhook *entities.SubscriptionWebhook) (*entities.SubscriptionWebhook, error)
}
