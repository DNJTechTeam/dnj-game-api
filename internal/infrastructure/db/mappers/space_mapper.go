package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapSpaceToEntity(model *models.Space) *entities.Space {
	if model == nil {
		return nil
	}
	return &entities.Space{ID: model.ID, Slug: model.Slug, Name: model.Name, MapReference: model.MapReference, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
}

func MapSpaceEntityToModel(entity *entities.Space) *models.Space {
	if entity == nil {
		return nil
	}
	return &models.Space{ID: entity.ID, Slug: entity.Slug, Name: entity.Name, MapReference: entity.MapReference, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}
}
