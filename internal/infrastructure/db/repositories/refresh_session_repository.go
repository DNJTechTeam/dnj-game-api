package repositories

import (
	"context"
	"errors"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RefreshSessionRepository struct {
	*BaseRepository[models.RefreshSession]
}

func NewRefreshSessionRepository(db *gorm.DB) interfaces.RefreshSessionRepositoryInterface {
	return &RefreshSessionRepository{BaseRepository: NewBaseRepository[models.RefreshSession](db)}
}

func (r *RefreshSessionRepository) Create(ctx context.Context, session *entities.RefreshSession) (*entities.RefreshSession, error) {
	model := mappers.MapEntityToRefreshSession(session)
	if err := r.BaseRepository.Create(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapRefreshSessionToEntity(model), nil
}

func (r *RefreshSessionRepository) FindByTokenHashForUpdate(ctx context.Context, tokenHash string) (*entities.RefreshSession, error) {
	var model models.RefreshSession
	err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", tokenHash).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrNotFound
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	return mappers.MapRefreshSessionToEntity(&model), nil
}

func (r *RefreshSessionRepository) Update(ctx context.Context, session *entities.RefreshSession) (*entities.RefreshSession, error) {
	model := mappers.MapEntityToRefreshSession(session)
	if err := r.BaseRepository.Update(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapRefreshSessionToEntity(model), nil
}

func (r *RefreshSessionRepository) RevokeFamily(ctx context.Context, familyID string, reuseDetected bool) error {
	now := time.Now().UTC()
	if reuseDetected {
		result := r.getDB(ctx).Model(&models.RefreshSession{}).
			Where("family_id = ?", familyID).
			Update("reuse_detected_at", now)
		if err := handleRepositoryError(result.Error); err != nil {
			return err
		}
	}
	result := r.getDB(ctx).Model(&models.RefreshSession{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", now)
	return handleRepositoryError(result.Error)
}
