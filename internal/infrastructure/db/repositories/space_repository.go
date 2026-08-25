package repositories

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
	spaceInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/space/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SpaceRepository struct{ *BaseRepository[models.Space] }

func NewSpaceRepository(db *gorm.DB) spaceInterfaces.SpaceRepositoryInterface {
	return &SpaceRepository{BaseRepository: NewBaseRepository[models.Space](db)}
}

func (r *SpaceRepository) Create(ctx context.Context, space *entities.Space) (*entities.Space, error) {
	row := mappers.MapSpaceEntityToModel(space)
	if err := r.BaseRepository.Create(ctx, row); err != nil {
		return nil, err
	}
	return mappers.MapSpaceToEntity(row), nil
}

func (r *SpaceRepository) FindByIDForUpdate(ctx context.Context, id string) (*entities.Space, error) {
	var row models.Space
	if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapSpaceToEntity(&row), nil
}

func (r *SpaceRepository) Update(ctx context.Context, space *entities.Space) (*entities.Space, error) {
	row := mappers.MapSpaceEntityToModel(space)
	if err := r.BaseRepository.Update(ctx, row); err != nil {
		return nil, err
	}
	return mappers.MapSpaceToEntity(row), nil
}

func (r *SpaceRepository) List(ctx context.Context, page uint64) (*messages.PaginatedResponse[entities.Space], error) {
	const limit = 20
	var rows []models.Space
	err := r.getDB(ctx).Order("name ASC").Order("id ASC").Limit(limit + 1).Offset(int(page) * limit).Find(&rows).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}
	items := make([]entities.Space, len(rows))
	for index := range rows {
		items[index] = *mappers.MapSpaceToEntity(&rows[index])
	}
	return &messages.PaginatedResponse[entities.Space]{Data: items, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: limit}}, nil
}
