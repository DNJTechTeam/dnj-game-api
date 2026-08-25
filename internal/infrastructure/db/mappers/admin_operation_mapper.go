package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapAdminOperationToEntity(model *models.AdminOperation) *entities.AdminOperation {
	if model == nil {
		return nil
	}
	return &entities.AdminOperation{ID: model.ID, ActorUserID: model.ActorUserID, IdempotencyKey: model.IdempotencyKey, Operation: model.Operation, EntityType: model.EntityType, EntityRef: model.EntityRef, RequestHash: model.RequestHash, HTTPStatus: model.HTTPStatus, Response: model.Response, CreatedAt: model.CreatedAt}
}

func MapAdminOperationEntityToModel(entity *entities.AdminOperation) *models.AdminOperation {
	if entity == nil {
		return nil
	}
	return &models.AdminOperation{ID: entity.ID, ActorUserID: entity.ActorUserID, IdempotencyKey: entity.IdempotencyKey, Operation: entity.Operation, EntityType: entity.EntityType, EntityRef: entity.EntityRef, RequestHash: entity.RequestHash, HTTPStatus: entity.HTTPStatus, Response: entity.Response, CreatedAt: entity.CreatedAt}
}
