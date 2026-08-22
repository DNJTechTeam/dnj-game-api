package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/identity/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapGoogleIdentityToEntity(model *models.GoogleIdentity) *entities.GoogleIdentity {
	if model == nil {
		return nil
	}
	return &entities.GoogleIdentity{
		ID: model.ID, UserID: model.UserID, Provider: model.Provider,
		Subject: model.Subject, Email: model.Email, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
}

func MapEntityToGoogleIdentity(entity *entities.GoogleIdentity) *models.GoogleIdentity {
	if entity == nil {
		return nil
	}
	return &models.GoogleIdentity{
		ID: entity.ID, UserID: entity.UserID, Provider: entity.Provider,
		Subject: entity.Subject, Email: entity.Email, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}
