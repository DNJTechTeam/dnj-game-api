package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapGroupInviteToEntity(model *models.GroupInvite) *entities.GroupInvite {
	if model == nil {
		return nil
	}
	return &entities.GroupInvite{ID: model.ID, GroupID: model.GroupID, CodeHash: model.CodeHash, ExpiresAt: model.ExpiresAt, RevokedAt: model.RevokedAt, ConsumedAt: model.ConsumedAt, ConsumedByUserID: model.ConsumedByUserID, CreatedByUserID: model.CreatedByUserID, ReplacesInviteID: model.ReplacesInviteID, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
}

func MapGroupInviteEntityToModel(entity *entities.GroupInvite) *models.GroupInvite {
	if entity == nil {
		return nil
	}
	return &models.GroupInvite{ID: entity.ID, GroupID: entity.GroupID, CodeHash: entity.CodeHash, ExpiresAt: entity.ExpiresAt, RevokedAt: entity.RevokedAt, ConsumedAt: entity.ConsumedAt, ConsumedByUserID: entity.ConsumedByUserID, CreatedByUserID: entity.CreatedByUserID, ReplacesInviteID: entity.ReplacesInviteID, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}
}
