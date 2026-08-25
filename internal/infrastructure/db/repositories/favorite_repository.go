package repositories

import (
	"context"
	"errors"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
	favoriteInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FavoriteRepository struct {
	*BaseRepository[models.UserFavorite]
}

func NewFavoriteRepository(db *gorm.DB) favoriteInterfaces.FavoriteRepositoryInterface {
	return &FavoriteRepository{BaseRepository: NewBaseRepository[models.UserFavorite](db)}
}

func (r *FavoriteRepository) ListVisible(
	ctx context.Context,
	userID uint64,
	generatedAt time.Time,
	page uint64,
) (*messages.PaginatedResponse[activityEntities.PublicActivity], error) {
	const limit = 10
	query := publiclyVisibleActivities(
		publicActivityQuery(
			r.getDB(ctx),
		).Joins("JOIN user_favorites ON user_favorites.activity_id = activities.id AND user_favorites.user_id = ?", userID),
		generatedAt,
	)
	var rows []publicActivityRow
	if err := orderPublicActivities(query).Limit(limit + 1).Offset(int(page) * limit).Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}
	items := make([]activityEntities.PublicActivity, len(rows))
	for index := range rows {
		items[index] = mapPublicActivityRow(&rows[index])
	}
	return &messages.PaginatedResponse[activityEntities.PublicActivity]{
		Data: items,
		Pagination: messages.Pagination{
			CurrentPage: messages.Uint64StringFromUint64(page + 1),
			HasNextPage: hasNext,
			Limit:       limit,
		},
	}, nil
}

func (r *FavoriteRepository) Create(ctx context.Context, favorite *entities.Favorite) (bool, error) {
	row := mappers.MapFavoriteEntityToModel(favorite)
	result := r.getDB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *FavoriteRepository) Delete(ctx context.Context, userID uint64, activityID string) (bool, error) {
	result := r.getDB(ctx).Where("user_id = ? AND activity_id = ?", userID, activityID).Delete(&models.UserFavorite{})
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *FavoriteRepository) FindOperation(
	ctx context.Context,
	actorUserID uint64,
	idempotencyKey string,
) (*entities.ParticipantOperation, error) {
	var row models.ParticipantOperation
	err := r.getDB(ctx).
		Where("actor_user_id = ? AND idempotency_key = ?", actorUserID, idempotencyKey).
		First(&row).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrNotFound
	}
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapParticipantOperationToEntity(&row), nil
}

func (r *FavoriteRepository) CreateOperation(
	ctx context.Context,
	operation *entities.ParticipantOperation,
) (*entities.ParticipantOperation, error) {
	resourceRef := operation.ActivityID
	if err := reserveGlobalIdempotencyKey(
		ctx,
		r.getDB(ctx),
		operation.ID,
		operation.ActorUserID,
		operation.IdempotencyKey,
		operation.Operation,
		&resourceRef,
		operation.IntentHash,
		operation.ResultRef,
		operation.HTTPStatus,
		operation.CreatedAt,
	); err != nil {
		return nil, err
	}
	row := mappers.MapParticipantOperationEntityToModel(operation)
	if err := r.getDB(ctx).Create(row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapParticipantOperationToEntity(row), nil
}
