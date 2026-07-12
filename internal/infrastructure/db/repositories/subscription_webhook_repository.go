package repositories

import (
	"context"

	swEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/entities"
	swInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"gorm.io/gorm"
)

type SubscriptionWebhookRepository struct {
	*BaseRepository[models.SubscriptionWebhook]
}

func NewSubscriptionWebhookRepository(db *gorm.DB) swInterfaces.SubscriptionWebhookRepositoryInterface {
	return &SubscriptionWebhookRepository{
		BaseRepository: NewBaseRepository[models.SubscriptionWebhook](db),
	}
}

func (r *SubscriptionWebhookRepository) Create(ctx context.Context, webhook *swEntities.SubscriptionWebhook) (*swEntities.SubscriptionWebhook, error) {
	model := mappers.MapSubscriptionWebhookEntityToModel(webhook)
	if err := r.BaseRepository.Create(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapSubscriptionWebhookToEntity(model), nil
}
