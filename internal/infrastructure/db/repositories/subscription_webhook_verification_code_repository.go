package repositories

import (
	"context"
	"errors"

	svcEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/entities"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"gorm.io/gorm"
)

type SubscriptionWebhookVerificationCodeRepository struct {
	*BaseRepository[models.SubscriptionWebhookVerificationCode]
}

func NewSubscriptionWebhookVerificationCodeRepository(db *gorm.DB) svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface {
	return &SubscriptionWebhookVerificationCodeRepository{
		BaseRepository: NewBaseRepository[models.SubscriptionWebhookVerificationCode](db),
	}
}

func (r *SubscriptionWebhookVerificationCodeRepository) Create(ctx context.Context, code *svcEntities.SubscriptionWebhookVerificationCode) (*svcEntities.SubscriptionWebhookVerificationCode, error) {
	model := mappers.MapSubscriptionWebhookVerificationCodeEntityToModel(code)
	if err := r.BaseRepository.Create(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapSubscriptionWebhookVerificationCodeToEntity(model), nil
}

func (r *SubscriptionWebhookVerificationCodeRepository) FindByEmail(ctx context.Context, email string) (*svcEntities.SubscriptionWebhookVerificationCode, error) {
	var model models.SubscriptionWebhookVerificationCode
	err := r.getDB(ctx).Where("email = ?", email).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, handleRepositoryError(err)
	}
	return mappers.MapSubscriptionWebhookVerificationCodeToEntity(&model), nil
}

func (r *SubscriptionWebhookVerificationCodeRepository) FindByDocument(ctx context.Context, document string) (*svcEntities.SubscriptionWebhookVerificationCode, error) {
	var model models.SubscriptionWebhookVerificationCode
	err := r.getDB(ctx).Where("document = ?", document).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, handleRepositoryError(err)
	}
	return mappers.MapSubscriptionWebhookVerificationCodeToEntity(&model), nil
}

func (r *SubscriptionWebhookVerificationCodeRepository) Update(ctx context.Context, code *svcEntities.SubscriptionWebhookVerificationCode) (*svcEntities.SubscriptionWebhookVerificationCode, error) {
	model := mappers.MapSubscriptionWebhookVerificationCodeEntityToModel(code)
	if err := r.BaseRepository.Update(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapSubscriptionWebhookVerificationCodeToEntity(model), nil
}

func (r *SubscriptionWebhookVerificationCodeRepository) FindByEmailAndCode(ctx context.Context, email string, verificationCode string) (*svcEntities.SubscriptionWebhookVerificationCode, error) {
	var model models.SubscriptionWebhookVerificationCode
	err := r.getDB(ctx).Where("email = ? AND verification_code = ?", email, verificationCode).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, handleRepositoryError(err)
	}
	return mappers.MapSubscriptionWebhookVerificationCodeToEntity(&model), nil
}
