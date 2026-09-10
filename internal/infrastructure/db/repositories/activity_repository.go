package repositories

import (
	"context"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
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
		query = query.Joins("JOIN users ON users.id = ?", actorUserID).Where(managerScopeActivityPredicate, actorUserID)
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

func (r *ActivityRepository) List(ctx context.Context, page uint64) (*messages.PaginatedResponse[entities.Activity], error) {
	return r.list(ctx, page, "", "")
}

func (r *ActivityRepository) ListAdmin(ctx context.Context, page uint64, kind, status string) (*messages.PaginatedResponse[entities.Activity], error) {
	return r.list(ctx, page, kind, status)
}

func (r *ActivityRepository) list(ctx context.Context, page uint64, kind, status string) (*messages.PaginatedResponse[entities.Activity], error) {
	const limit = 20
	var rows []models.Activity
	query := r.getDB(ctx)
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	countQuery := r.getDB(ctx).Model(&models.Activity{})
	if kind != "" {
		countQuery = countQuery.Where("kind = ?", kind)
	}
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	err := query.Order("name ASC").Order("id ASC").Limit(limit + 1).Offset(int(page) * limit).Find(&rows).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}
	items := make([]entities.Activity, len(rows))
	for index := range rows {
		items[index] = *mappers.MapActivityToEntity(&rows[index])
	}
	return &messages.PaginatedResponse[entities.Activity]{Data: items, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: limit, Total: int(total)}}, nil
}

func (r *ActivityRepository) Create(ctx context.Context, activity *entities.Activity) (*entities.Activity, error) {
	row := mappers.MapActivityEntityToModel(activity)
	if err := r.BaseRepository.Create(ctx, row); err != nil {
		return nil, err
	}
	return mappers.MapActivityToEntity(row), nil
}

func (r *ActivityRepository) FindByID(ctx context.Context, activityID string) (*entities.Activity, error) {
	var row models.Activity
	if err := r.getDB(ctx).Where("id = ?", activityID).First(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapActivityToEntity(&row), nil
}

func (r *ActivityRepository) FindByIDForUpdate(ctx context.Context, activityID string) (*entities.Activity, error) {
	var row models.Activity
	if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", activityID).First(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapActivityToEntity(&row), nil
}

func (r *ActivityRepository) HasScheduleOverlap(ctx context.Context, spaceID string, startsAt, endsAt time.Time, excludeActivityID *string) (bool, error) {
	query := r.getDB(ctx).Model(&models.Activity{}).
		Where("kind = ? AND status <> ? AND space_id = ? AND starts_at < ? AND ends_at > ?", string(entities.KindSchedule), string(entities.StatusArchived), spaceID, endsAt.UTC(), startsAt.UTC())
	if excludeActivityID != nil {
		query = query.Where("id <> ?", *excludeActivityID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, handleRepositoryError(err)
	}
	return count > 0, nil
}

func (r *ActivityRepository) FindScheduleForSpaceAt(ctx context.Context, spaceID string, now time.Time) (*entities.Activity, error) {
	var row models.Activity
	if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("kind = ? AND status <> ? AND space_id = ? AND starts_at <= ? AND ends_at > ?", string(entities.KindSchedule), string(entities.StatusArchived), spaceID, now.UTC(), now.UTC()).
		Order("starts_at ASC").Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapActivityToEntity(&row), nil
}

func (r *ActivityRepository) Update(ctx context.Context, activity *entities.Activity) (*entities.Activity, error) {
	row := mappers.MapActivityEntityToModel(activity)
	if err := r.BaseRepository.Update(ctx, row); err != nil {
		return nil, err
	}
	return mappers.MapActivityToEntity(row), nil
}

func (r *ActivityRepository) ListManagers(ctx context.Context, activityID string, page uint64) (*messages.PaginatedResponse[userEntities.User], error) {
	const limit = 20
	var rows []models.User
	err := r.getDB(ctx).Model(&models.User{}).
		Joins("JOIN activity_manager_assignments ON activity_manager_assignments.user_id = users.id").
		Where("activity_manager_assignments.activity_id = ?", activityID).
		Order("users.name ASC").Order("users.id ASC").Limit(limit + 1).Offset(int(page) * limit).Find(&rows).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}
	items := make([]userEntities.User, len(rows))
	for index := range rows {
		items[index] = *mappers.MapUserToEntity(&rows[index])
	}
	return &messages.PaginatedResponse[userEntities.User]{Data: items, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: limit}}, nil
}

func (r *ActivityRepository) CreateManagerAssignment(ctx context.Context, assignment *entities.ManagerAssignment) (bool, error) {
	row := &models.ActivityManagerAssignment{ActivityID: assignment.ActivityID, UserID: assignment.UserID, CreatedAt: assignment.CreatedAt}
	result := r.getDB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *ActivityRepository) DeleteManagerAssignment(ctx context.Context, activityID string, userID uint64) (bool, error) {
	result := r.getDB(ctx).Where("activity_id = ? AND user_id = ?", activityID, userID).Delete(&models.ActivityManagerAssignment{})
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *ActivityRepository) CountManagerAssignments(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	if err := r.getDB(ctx).Model(&models.ActivityManagerAssignment{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, handleRepositoryError(err)
	}
	return count, nil
}
