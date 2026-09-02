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
	"github.com/google/uuid"
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
	// Batched to stay under Postgres's 65535 bind-parameter limit on a single
	// INSERT — models.Notification has enough columns that an unbatched
	// broadcast could hit it once the eligible user base grows large.
	if err := r.getDB(ctx).CreateInBatches(rows, 500).Error; err != nil {
		return handleRepositoryError(err)
	}
	_, err := r.CreatePendingDeliveries(ctx, notifications, time.Now().UTC())
	return err
}

func (r *NotificationRepository) UpsertPushSubscription(
	ctx context.Context,
	subscription *entities.PushSubscription,
) (*entities.PushSubscription, error) {
	row := &models.PushSubscription{
		ID: subscription.ID, UserID: subscription.UserID, Endpoint: subscription.Endpoint,
		P256DH: subscription.P256DH, Auth: subscription.Auth, State: subscription.State,
		CreatedAt: subscription.CreatedAt, UpdatedAt: subscription.UpdatedAt, DisabledAt: subscription.DisabledAt,
	}
	err := r.getDB(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.Assignments(map[string]any{
			"user_id": subscription.UserID, "p256dh": subscription.P256DH, "auth": subscription.Auth,
			"state": subscription.State, "updated_at": subscription.UpdatedAt, "disabled_at": nil,
		}),
	}).Create(row).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	var saved models.PushSubscription
	if err := r.getDB(ctx).Where("endpoint = ?", subscription.Endpoint).Take(&saved).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return &entities.PushSubscription{ID: saved.ID, UserID: saved.UserID, Endpoint: saved.Endpoint, P256DH: saved.P256DH, Auth: saved.Auth, State: saved.State, CreatedAt: saved.CreatedAt, UpdatedAt: saved.UpdatedAt, DisabledAt: saved.DisabledAt}, nil
}

func (r *NotificationRepository) DeactivatePushSubscription(ctx context.Context, userID uint64, endpoint string, now time.Time) error {
	return handleRepositoryError(r.getDB(ctx).Model(&models.PushSubscription{}).
		Where("user_id = ? AND endpoint = ? AND state = ?", userID, endpoint, "active").
		Updates(map[string]any{"state": "inactive", "disabled_at": now, "updated_at": now}).Error)
}

func (r *NotificationRepository) CreateQueueCall(ctx context.Context, notification *entities.Notification, now time.Time) (bool, error) {
	row := mappers.MapNotificationEntityToModel(notification)
	result := r.getDB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	if result.RowsAffected == 0 {
		var existing models.Notification
		if err := r.getDB(ctx).Where("user_id = ? AND source_type = ? AND source_id = ?", notification.UserID, notification.SourceType, notification.SourceID).Take(&existing).Error; err != nil {
			return false, handleRepositoryError(err)
		}
		notification.ID = existing.ID
		return false, nil
	}
	_, err := r.CreatePendingDeliveries(ctx, []*entities.Notification{notification}, now)
	return true, err
}

// CreatePendingDeliveries materializes the transaction outbox. It never calls
// a push provider, keeping domain writes independent from browser delivery.
func (r *NotificationRepository) CreatePendingDeliveries(ctx context.Context, notifications []*entities.Notification, now time.Time) (int, error) {
	if len(notifications) == 0 {
		return 0, nil
	}
	userIDs := make([]uint64, 0, len(notifications))
	byUser := make(map[uint64][]string, len(notifications))
	for _, notification := range notifications {
		if _, ok := byUser[notification.UserID]; !ok {
			userIDs = append(userIDs, notification.UserID)
		}
		byUser[notification.UserID] = append(byUser[notification.UserID], notification.ID)
	}
	var subscriptions []models.PushSubscription
	if err := r.getDB(ctx).Where("user_id IN ? AND state = ?", userIDs, "active").Find(&subscriptions).Error; err != nil {
		return 0, handleRepositoryError(err)
	}
	deliveries := make([]*models.NotificationDelivery, 0)
	for _, subscription := range subscriptions {
		for _, notificationID := range byUser[subscription.UserID] {
			deliveries = append(deliveries, &models.NotificationDelivery{ID: uuid.NewString(), NotificationID: notificationID, SubscriptionID: subscription.ID, State: "pending", CreatedAt: now, UpdatedAt: now})
		}
	}
	if len(deliveries) == 0 {
		return 0, nil
	}
	err := r.getDB(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "notification_id"}, {Name: "subscription_id"}}, DoNothing: true}).CreateInBatches(deliveries, 500).Error
	if err != nil {
		return 0, handleRepositoryError(err)
	}
	return len(deliveries), nil
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
