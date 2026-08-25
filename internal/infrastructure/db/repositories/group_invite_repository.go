package repositories

import (
	"context"
	"errors"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

type GroupInviteRepository struct {
	*BaseRepository[models.GroupInvite]
}

func NewGroupInviteRepository(db *gorm.DB) interfaces.GroupInviteRepositoryInterface {
	return &GroupInviteRepository{BaseRepository: NewBaseRepository[models.GroupInvite](db)}
}

func (r *GroupInviteRepository) Create(ctx context.Context, invite *entities.GroupInvite) (*entities.GroupInvite, error) {
	model := mappers.MapGroupInviteEntityToModel(invite)
	if err := r.BaseRepository.Create(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapGroupInviteToEntity(model), nil
}

func (r *GroupInviteRepository) FindByIDAndGroup(ctx context.Context, inviteID uint64, groupID uint64) (*entities.GroupInvite, error) {
	var model models.GroupInvite
	err := r.getDB(ctx).Where("id = ? AND group_id = ?", inviteID, groupID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrNotFound
	}
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapGroupInviteToEntity(&model), nil
}

func (r *GroupInviteRepository) FindByHash(ctx context.Context, codeHash string) (*entities.GroupInvite, error) {
	var model models.GroupInvite
	err := r.getDB(ctx).Where("code_hash = ?", codeHash).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrNotFound
	}
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapGroupInviteToEntity(&model), nil
}

func (r *GroupInviteRepository) ConsumeAvailable(ctx context.Context, inviteID uint64, userID uint64, now time.Time) (bool, error) {
	result := r.getDB(ctx).Model(&models.GroupInvite{}).
		Where("id = ? AND revoked_at IS NULL AND consumed_at IS NULL AND expires_at > ?", inviteID, now).
		Updates(map[string]any{"consumed_at": now, "consumed_by_user_id": userID, "updated_at": now})
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *GroupInviteRepository) RevokeAvailable(ctx context.Context, inviteID uint64, groupID uint64, now time.Time) (bool, error) {
	result := r.getDB(ctx).Model(&models.GroupInvite{}).
		Where("id = ? AND group_id = ? AND revoked_at IS NULL AND consumed_at IS NULL", inviteID, groupID).
		Updates(map[string]any{"revoked_at": now, "updated_at": now})
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *GroupInviteRepository) ListByGroup(ctx context.Context, groupID uint64, page uint64) (*messages.PaginatedResponse[entities.GroupInvite], error) {
	const limit = 10
	var modelsList []models.GroupInvite
	err := r.getDB(ctx).Where("group_id = ?", groupID).
		Order("created_at DESC").Order("id DESC").Limit(limit + 1).Offset(int(page) * limit).Find(&modelsList).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(modelsList) > limit
	if hasNext {
		modelsList = modelsList[:limit]
	}
	items := make([]entities.GroupInvite, len(modelsList))
	for i := range modelsList {
		items[i] = *mappers.MapGroupInviteToEntity(&modelsList[i])
	}
	return &messages.PaginatedResponse[entities.GroupInvite]{Data: items, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: limit}}, nil
}
