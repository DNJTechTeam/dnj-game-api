package repositories

import (
	"context"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ActivityRepository struct {
	*BaseRepository[models.Activity]
}

func NewActivityRepository(db *gorm.DB) activityInterfaces.ActivityRepositoryInterface {
	return &ActivityRepository{BaseRepository: NewBaseRepository[models.Activity](db)}
}

func (r *ActivityRepository) FindAuthorizedForUpdate(ctx context.Context, activityID string, actorUserID uint64, global bool) (*entities.Activity, error) {
	var row models.Activity
	query := r.getDB(ctx).Model(&models.Activity{}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("activities.id = ?", activityID)
	if !global {
		query = query.Joins("JOIN activity_manager_assignments ON activity_manager_assignments.activity_id = activities.id AND activity_manager_assignments.user_id = ?", actorUserID)
	}
	if err := query.First(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapActivityToEntity(&row), nil
}

func (r *ActivityRepository) TransitionStatus(ctx context.Context, activityID string, from, to entities.Status, updatedAt time.Time) error {
	result := r.getDB(ctx).Model(&models.Activity{}).Where("id = ? AND status = ?", activityID, string(from)).Updates(map[string]any{"status": string(to), "updated_at": updatedAt})
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	return nil
}
