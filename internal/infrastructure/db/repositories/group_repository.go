package repositories

import (
	"context"
	"errors"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"gorm.io/gorm"
)

type GroupRepository struct {
	*BaseRepository[models.Group]
}

func NewGroupRepository(db *gorm.DB) groupInterfaces.GroupRepositoryInterface {
	return &GroupRepository{
		BaseRepository: NewBaseRepository[models.Group](db),
	}
}

func (r *GroupRepository) Create(ctx context.Context, group *groupEntities.Group) (*groupEntities.Group, error) {
	model := mappers.MapGroupEntityToModel(group)
	if err := r.BaseRepository.Create(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapGroupToEntity(model), nil
}

func (r *GroupRepository) FindByID(ctx context.Context, id uint64) (*groupEntities.Group, error) {
	model, err := r.BaseRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mappers.MapGroupToEntity(model), nil
}

func (r *GroupRepository) FindByNameExact(ctx context.Context, name string) (*groupEntities.Group, error) {
	var model models.Group
	err := r.getDB(ctx).Where("name ILIKE ?", name).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, handleRepositoryError(err)
	}
	return mappers.MapGroupToEntity(&model), nil
}

func (r *GroupRepository) Search(ctx context.Context, query string, limit int) ([]*groupEntities.Group, error) {
	var groupModels []models.Group
	err := r.getDB(ctx).Where("name ILIKE ?", "%"+query+"%").Order("name ASC").Order("id ASC").Limit(limit).Find(&groupModels).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	groups := make([]*groupEntities.Group, len(groupModels))
	for i := range groupModels {
		groups[i] = mappers.MapGroupToEntity(&groupModels[i])
	}
	return groups, nil
}

func (r *GroupRepository) SearchPage(ctx context.Context, query string, page uint64) (*messages.PaginatedResponse[groupEntities.Group], error) {
	const limit = 20
	var groupModels []models.Group
	db := r.getDB(ctx)
	if query != "" {
		db = db.Where("name ILIKE ?", "%"+query+"%")
	}
	err := db.Order("name ASC").Order("id ASC").
		Limit(limit + 1).Offset(int(page) * limit).
		Find(&groupModels).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(groupModels) > limit
	if hasNext {
		groupModels = groupModels[:limit]
	}
	groups := make([]groupEntities.Group, len(groupModels))
	for i := range groupModels {
		groups[i] = *mappers.MapGroupToEntity(&groupModels[i])
	}
	return &messages.PaginatedResponse[groupEntities.Group]{Data: groups, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: limit}}, nil
}

func (r *GroupRepository) ExistsByID(ctx context.Context, id uint64) bool {
	return r.BaseRepository.ExistsBy(ctx, map[string]interface{}{"id": id})
}
