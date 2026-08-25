package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
	notificationInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotificationRepository struct {
	*BaseRepository[models.Notification]
}

func NewNotificationRepository(db *gorm.DB) notificationInterfaces.Repository {
	return &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](db)}
}

func (r *NotificationRepository) FindPreferences(ctx context.Context, userID uint64) (*entities.Preferences, error) {
	var row models.NotificationPreference
	if err := r.getDB(ctx).Where("user_id = ?", userID).Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapNotificationPreferenceToEntity(&row), nil
}

func (r *NotificationRepository) UpsertPreferences(
	ctx context.Context,
	prefs *entities.Preferences,
) (*entities.Preferences, error) {
	row := &models.NotificationPreference{
		UserID:              prefs.UserID,
		PointsEnabled:       prefs.PointsEnabled,
		AnnouncementEnabled: prefs.AnnouncementEnabled,
		UpdatedAt:           prefs.UpdatedAt,
	}
	err := r.getDB(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"points_enabled", "announcement_enabled", "updated_at"}),
		}).
		Create(row).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapNotificationPreferenceToEntity(row), nil
}

func (r *NotificationRepository) List(
	ctx context.Context,
	userID uint64,
	page uint64,
) (*messages.PaginatedResponse[entities.Notification], error) {
	const limit = 10
	offset := int(page) * limit

	var rows []models.Notification
	err := r.getDB(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Limit(limit + 1).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}

	hasNextPage := len(rows) > limit
	if hasNextPage {
		rows = rows[:limit]
	}

	items := make([]entities.Notification, len(rows))
	for i := range rows {
		items[i] = *mappers.MapNotificationToEntity(&rows[i])
	}
	return &messages.PaginatedResponse[entities.Notification]{
		Data: items,
		Pagination: messages.Pagination{
			CurrentPage: messages.Uint64StringFromUint64(page + 1),
			HasNextPage: hasNextPage,
			Limit:       limit,
		},
	}, nil
}

func (r *NotificationRepository) CountUnread(ctx context.Context, userID uint64) (uint64, error) {
	var count int64
	err := r.getDB(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND state = ?", userID, string(entities.StateUnread)).
		Count(&count).Error
	if err != nil {
		return 0, handleRepositoryError(err)
	}
	return uint64(count), nil
}

func (r *NotificationRepository) FindByIDAndUser(
	ctx context.Context,
	id string,
	userID uint64,
) (*entities.Notification, error) {
	var row models.Notification
	if err := r.getDB(ctx).Where("id = ? AND user_id = ?", id, userID).Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapNotificationToEntity(&row), nil
}

func (r *NotificationRepository) MarkRead(
	ctx context.Context,
	id string,
	userID uint64,
	now time.Time,
) (*entities.Notification, error) {
	result := r.getDB(ctx).
		Model(&models.Notification{}).
		Where("id = ? AND user_id = ? AND state = ?", id, userID, string(entities.StateUnread)).
		Updates(map[string]any{"state": string(entities.StateRead), "read_at": now})
	if result.Error != nil {
		return nil, handleRepositoryError(result.Error)
	}
	return r.FindByIDAndUser(ctx, id, userID)
}

func (r *NotificationRepository) ResolveAnnouncementRecipients(
	ctx context.Context,
	explicitUserIDs []uint64,
) ([]uint64, error) {
	query := r.getDB(ctx).
		Table("users").
		Joins("LEFT JOIN notification_preferences ON notification_preferences.user_id = users.id").
		Where("users.role = ? AND users.onboarding_complete = ? AND users.deleted_at IS NULL", "DEFAULT", true).
		Where("COALESCE(notification_preferences.announcement_enabled, true) = true")
	if len(explicitUserIDs) > 0 {
		query = query.Where("users.id IN ?", explicitUserIDs)
	}

	var ids []uint64
	if err := query.Pluck("users.id", &ids).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return ids, nil
}

func (r *NotificationRepository) CreateBroadcast(ctx context.Context, notifications []*entities.Notification) error {
	if len(notifications) == 0 {
		return nil
	}
	rows := make([]*models.Notification, len(notifications))
	for i, item := range notifications {
		rows[i] = mappers.MapNotificationEntityToModel(item)
	}
	return handleRepositoryError(r.getDB(ctx).Create(rows).Error)
}

func (r *NotificationRepository) FindOperation(
	ctx context.Context,
	actor uint64,
	key string,
) (*entities.Operation, error) {
	var row models.IdempotencyOperation
	if err := r.getDB(ctx).Where("actor_user_id = ? AND idempotency_key = ?", actor, key).First(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapNotificationOperationToEntity(&row), nil
}

func (r *NotificationRepository) CreateOperation(ctx context.Context, operation *entities.Operation) error {
	snapshot := json.RawMessage(operation.ResponseSnapshot)
	if len(snapshot) == 0 {
		snapshot = json.RawMessage(`{}`)
	}
	row := &models.IdempotencyOperation{
		ID:               operation.ID,
		ActorUserID:      operation.ActorUserID,
		IdempotencyKey:   operation.IdempotencyKey,
		Operation:        operation.Operation,
		ResourceRef:      operation.ResourceRef,
		IntentHash:       operation.IntentHash,
		State:            operation.State,
		ResultRef:        operation.ResultRef,
		ResultCount:      operation.ResultCount,
		ResponseSnapshot: snapshot,
		HTTPStatus:       operation.HTTPStatus,
		CreatedAt:        operation.CreatedAt,
		CompletedAt:      operation.CompletedAt,
	}
	return handleRepositoryError(r.getDB(ctx).Create(row).Error)
}
