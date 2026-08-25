package repositories

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func (r *SpaceRepository) FindByID(ctx context.Context, id string) (*entities.Space, error) {
	var row models.Space
	if err := r.getDB(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapSpaceToEntity(&row), nil
}
