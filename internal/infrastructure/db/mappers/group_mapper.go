package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapGroupToEntity(group *models.Group) *entities.Group {
	if group == nil {
		return nil
	}

	return &entities.Group{
		ID:   group.ID,
		Name: group.Name,
	}
}

func MapGroupEntityToModel(group *entities.Group) *models.Group {
	if group == nil {
		return nil
	}

	return &models.Group{
		ID:   group.ID,
		Name: group.Name,
	}
}
