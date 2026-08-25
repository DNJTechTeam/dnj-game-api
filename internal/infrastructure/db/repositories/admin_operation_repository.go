package repositories

import (
	"context"
	"errors"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/entities"
	adminInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

type AdminOperationRepository struct {
	*BaseRepository[models.AdminOperation]
}

func NewAdminOperationRepository(db *gorm.DB) adminInterfaces.AdminOperationRepositoryInterface {
	return &AdminOperationRepository{BaseRepository: NewBaseRepository[models.AdminOperation](db)}
}

func (r *AdminOperationRepository) FindByActorAndIdempotencyKey(
	ctx context.Context,
	actorUserID uint64,
	key string,
) (*entities.AdminOperation, error) {
	var row models.AdminOperation
	err := r.getDB(ctx).Where("actor_user_id = ? AND idempotency_key = ?", actorUserID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrNotFound
	}
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapAdminOperationToEntity(&row), nil
}

func (r *AdminOperationRepository) Create(
	ctx context.Context,
	operation *entities.AdminOperation,
) (*entities.AdminOperation, error) {
	resourceRef := operation.EntityRef
	if err := reserveGlobalIdempotencyKey(
		ctx,
		r.getDB(ctx),
		operation.ID,
		operation.ActorUserID,
		operation.IdempotencyKey,
		operation.Operation,
		&resourceRef,
		operation.RequestHash,
		&resourceRef,
		operation.HTTPStatus,
		operation.CreatedAt,
	); err != nil {
		return nil, err
	}
	row := mappers.MapAdminOperationEntityToModel(operation)
	if err := r.BaseRepository.Create(ctx, row); err != nil {
		return nil, err
	}
	return mappers.MapAdminOperationToEntity(row), nil
}
