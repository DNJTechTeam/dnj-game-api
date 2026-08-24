package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapFavoriteEntityToModel(entity *entities.Favorite) *models.UserFavorite {
	if entity == nil {
		return nil
	}
	return &models.UserFavorite{UserID: entity.UserID, ActivityID: entity.ActivityID, CreatedAt: entity.CreatedAt}
}

func MapParticipantOperationToEntity(model *models.ParticipantOperation) *entities.ParticipantOperation {
	if model == nil {
		return nil
	}
	return &entities.ParticipantOperation{ID: model.ID, ActorUserID: model.ActorUserID, IdempotencyKey: model.IdempotencyKey, Operation: model.Operation, ActivityID: model.ActivityID, IntentHash: model.IntentHash, HTTPStatus: model.HTTPStatus, CreatedAt: model.CreatedAt}
}

func MapParticipantOperationEntityToModel(entity *entities.ParticipantOperation) *models.ParticipantOperation {
	if entity == nil {
		return nil
	}
	return &models.ParticipantOperation{ID: entity.ID, ActorUserID: entity.ActorUserID, IdempotencyKey: entity.IdempotencyKey, Operation: entity.Operation, ActivityID: entity.ActivityID, IntentHash: entity.IntentHash, HTTPStatus: entity.HTTPStatus, CreatedAt: entity.CreatedAt}
}
