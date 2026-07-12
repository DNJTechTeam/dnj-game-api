package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapSubscriptionWebhookVerificationCodeToEntity(code *models.SubscriptionWebhookVerificationCode) *entities.SubscriptionWebhookVerificationCode {
	if code == nil {
		return nil
	}

	return &entities.SubscriptionWebhookVerificationCode{
		ID:                    code.ID,
		SubscriptionWebhookID: code.SubscriptionWebhookID,
		Email:                 code.Email,
		Name:                  code.Name,
		MobilePhone:           code.MobilePhone,
		Document:              code.Document,
		VerificationCode:      code.VerificationCode,
		Group:                 code.Group,
		UserID:                code.UserID,
		CreatedAt:             code.CreatedAt,
	}
}

func MapSubscriptionWebhookVerificationCodeEntityToModel(code *entities.SubscriptionWebhookVerificationCode) *models.SubscriptionWebhookVerificationCode {
	if code == nil {
		return nil
	}

	return &models.SubscriptionWebhookVerificationCode{
		ID:                    code.ID,
		SubscriptionWebhookID: code.SubscriptionWebhookID,
		Email:                 code.Email,
		Name:                  code.Name,
		MobilePhone:           code.MobilePhone,
		Document:              code.Document,
		VerificationCode:      code.VerificationCode,
		Group:                 code.Group,
		UserID:                code.UserID,
		CreatedAt:             code.CreatedAt,
	}
}
