package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapSubscriptionWebhookToEntity(webhook *models.SubscriptionWebhook) *entities.SubscriptionWebhook {
	if webhook == nil {
		return nil
	}

	return &entities.SubscriptionWebhook{
		ID:        webhook.ID,
		Payload:   webhook.Payload,
		CreatedAt: webhook.CreatedAt,
	}
}

func MapSubscriptionWebhookEntityToModel(webhook *entities.SubscriptionWebhook) *models.SubscriptionWebhook {
	if webhook == nil {
		return nil
	}

	return &models.SubscriptionWebhook{
		ID:        webhook.ID,
		Payload:   webhook.Payload,
		CreatedAt: webhook.CreatedAt,
	}
}
