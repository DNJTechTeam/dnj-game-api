package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/task/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapTaskToEntity(model *models.Task) *entities.Task {
	if model == nil {
		return nil
	}

	return &entities.Task{
		ID:          model.ID,
		UserID:      model.UserID,
		Title:       model.Title,
		Description: model.Description,
		Status:      entities.TaskStatus(model.Status),
		DueDate:     model.DueDate,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

func MapTaskEntityToModel(entity *entities.Task) *models.Task {
	if entity == nil {
		return nil
	}

	return &models.Task{
		ID:          entity.ID,
		UserID:      entity.UserID,
		Title:       entity.Title,
		Description: entity.Description,
		Status:      string(entity.Status),
		DueDate:     entity.DueDate,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}
