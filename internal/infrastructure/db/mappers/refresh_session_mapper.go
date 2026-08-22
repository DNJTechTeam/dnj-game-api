package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapRefreshSessionToEntity(model *models.RefreshSession) *entities.RefreshSession {
	if model == nil {
		return nil
	}
	return &entities.RefreshSession{
		ID: model.ID, UserID: model.UserID, FamilyID: model.FamilyID, TokenHash: model.TokenHash,
		ReplacedByHash: model.ReplacedByHash, ExpiresAt: model.ExpiresAt, RevokedAt: model.RevokedAt,
		ReuseDetectedAt: model.ReuseDetectedAt, CreatedAt: model.CreatedAt, LastUsedAt: model.LastUsedAt,
	}
}

func MapEntityToRefreshSession(entity *entities.RefreshSession) *models.RefreshSession {
	if entity == nil {
		return nil
	}
	return &models.RefreshSession{
		ID: entity.ID, UserID: entity.UserID, FamilyID: entity.FamilyID, TokenHash: entity.TokenHash,
		ReplacedByHash: entity.ReplacedByHash, ExpiresAt: entity.ExpiresAt, RevokedAt: entity.RevokedAt,
		ReuseDetectedAt: entity.ReuseDetectedAt, CreatedAt: entity.CreatedAt, LastUsedAt: entity.LastUsedAt,
	}
}
