package services

import (
	"context"
	"encoding/json"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	swEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/entities"
	swInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/interfaces"
	svcEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/entities"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
)

// SubscriptionWebhookService ingests webhooks from the external event
// subscription platform. The raw payload is always persisted first, even if
// translation fails, so no data received from the platform is ever lost.
type SubscriptionWebhookService struct {
	*BaseService
	subscriptionWebhookRepository swInterfaces.SubscriptionWebhookRepositoryInterface
	verificationCodeRepository    svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface
	translator                    interfaces.WebhookPayloadTranslatorInterface
}

func NewSubscriptionWebhookService(
	baseService *BaseService,
	subscriptionWebhookRepository swInterfaces.SubscriptionWebhookRepositoryInterface,
	verificationCodeRepository svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface,
	translator interfaces.WebhookPayloadTranslatorInterface,
) interfaces.SubscriptionWebhookServiceInterface {
	return &SubscriptionWebhookService{
		BaseService:                   baseService,
		subscriptionWebhookRepository: subscriptionWebhookRepository,
		verificationCodeRepository:    verificationCodeRepository,
		translator:                    translator,
	}
}

func (s *SubscriptionWebhookService) Ingest(ctx context.Context, rawPayload map[string]any) error {
	payloadJSON, err := json.Marshal(rawPayload)
	if err != nil {
		return appErrors.NewSimpleError("Payload de webhook inválido.")
	}

	webhook, err := s.subscriptionWebhookRepository.Create(ctx, &swEntities.SubscriptionWebhook{
		Payload: string(payloadJSON),
	})
	if err != nil {
		return appErrors.InternalError
	}

	translated, err := s.translator.Translate(rawPayload)
	if err != nil {
		return err
	}

	return s.WithTransaction(ctx, func(ctx context.Context) error {
		return s.upsertVerificationCode(ctx, webhook.ID, translated)
	})
}

func (s *SubscriptionWebhookService) upsertVerificationCode(ctx context.Context, webhookID uint64, translated *interfaces.TranslatedSubscription) error {
	existing, err := s.verificationCodeRepository.FindByEmail(ctx, translated.Email)
	if err != nil {
		return appErrors.InternalError
	}

	code := common.GenerateVerificationCode()

	if existing != nil {
		existing.SubscriptionWebhookID = webhookID
		existing.Name = translated.Name
		existing.MobilePhone = translated.MobilePhone
		existing.Document = translated.Document
		existing.Group = translated.Group
		existing.VerificationCode = code

		if _, err := s.verificationCodeRepository.Update(ctx, existing); err != nil {
			return appErrors.InternalError
		}
		return nil
	}

	_, err = s.verificationCodeRepository.Create(ctx, &svcEntities.SubscriptionWebhookVerificationCode{
		SubscriptionWebhookID: webhookID,
		Email:                 translated.Email,
		Name:                  translated.Name,
		MobilePhone:           translated.MobilePhone,
		Document:              translated.Document,
		VerificationCode:      code,
		Group:                 translated.Group,
	})
	if err != nil {
		return appErrors.InternalError
	}
	return nil
}
