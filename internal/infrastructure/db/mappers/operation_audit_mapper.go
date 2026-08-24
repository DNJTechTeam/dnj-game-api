package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapOperationAuditToEntity(model *models.OperationAudit) *entities.OperationAudit {
	if model == nil {
		return nil
	}
	return &entities.OperationAudit{ID: model.ID, ActorUserID: model.ActorUserID, Action: model.Action, EntityType: model.EntityType, EntityID: model.EntityID, Metadata: model.Metadata, IdempotencyKey: model.IdempotencyKey, CreatedAt: model.CreatedAt}
}

func MapOperationAuditEntityToModel(entity *entities.OperationAudit) *models.OperationAudit {
	if entity == nil {
		return nil
	}
	return &models.OperationAudit{ID: entity.ID, ActorUserID: entity.ActorUserID, Action: entity.Action, EntityType: entity.EntityType, EntityID: entity.EntityID, Metadata: entity.Metadata, IdempotencyKey: entity.IdempotencyKey, CreatedAt: entity.CreatedAt}
}
