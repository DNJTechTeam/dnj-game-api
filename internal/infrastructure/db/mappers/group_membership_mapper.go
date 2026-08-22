package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapGroupMembershipToEntity(model *models.GroupMembership) *entities.GroupMembership {
	if model == nil {
		return nil
	}
	return &entities.GroupMembership{ID: model.ID, UserID: model.UserID, GroupID: model.GroupID, JoinedAt: model.JoinedAt, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
}

func MapGroupMembershipEntityToModel(entity *entities.GroupMembership) *models.GroupMembership {
	if entity == nil {
		return nil
	}
	return &models.GroupMembership{ID: entity.ID, UserID: entity.UserID, GroupID: entity.GroupID, JoinedAt: entity.JoinedAt, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}
}
