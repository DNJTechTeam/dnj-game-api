package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/emailsignupcode/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapEmailSignupCodeToEntity(model *models.EmailSignupCode) *entities.EmailSignupCode {
	if model == nil {
		return nil
	}
	return &entities.EmailSignupCode{
		ID: model.ID, Email: model.Email, CodeHash: model.CodeHash, ExpiresAt: model.ExpiresAt,
		ConsumedAt: model.ConsumedAt, Attempts: model.Attempts, LastSentAt: model.LastSentAt,
		UserID: model.UserID, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
}

func MapEntityToEmailSignupCode(entity *entities.EmailSignupCode) *models.EmailSignupCode {
	if entity == nil {
		return nil
	}
	return &models.EmailSignupCode{
		ID: entity.ID, Email: entity.Email, CodeHash: entity.CodeHash, ExpiresAt: entity.ExpiresAt,
		ConsumedAt: entity.ConsumedAt, Attempts: entity.Attempts, LastSentAt: entity.LastSentAt,
		UserID: entity.UserID, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}
