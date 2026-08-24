package repositories

import (
	"context"
	"errors"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/entities"
	auditInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

type OperationAuditRepository struct {
	*BaseRepository[models.OperationAudit]
}

func NewOperationAuditRepository(db *gorm.DB) auditInterfaces.OperationAuditRepositoryInterface {
	return &OperationAuditRepository{BaseRepository: NewBaseRepository[models.OperationAudit](db)}
}

func (r *OperationAuditRepository) FindByActorAndIdempotencyKey(ctx context.Context, actorUserID uint64, key string) (*entities.OperationAudit, error) {
	var row models.OperationAudit
	err := r.getDB(ctx).Where("actor_user_id = ? AND idempotency_key = ?", actorUserID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrNotFound
	}
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapOperationAuditToEntity(&row), nil
}

func (r *OperationAuditRepository) Create(ctx context.Context, audit *entities.OperationAudit) (*entities.OperationAudit, error) {
	row := mappers.MapOperationAuditEntityToModel(audit)
	if err := r.BaseRepository.Create(ctx, row); err != nil {
		return nil, err
	}
	return mappers.MapOperationAuditToEntity(row), nil
}
