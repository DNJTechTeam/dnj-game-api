package repositories

import (
	"context"
	"time"

	notificationEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// writeDerivedNotification persists a server-derived notification inside the
// caller's transaction. moment_moderation can never be silenced; points
// honors the recipient's stored preference (enabled by default when the user
// never customized it). Failures roll back with the rest of the transaction,
// keeping the notification and the event that caused it atomic.
func writeDerivedNotification(
	ctx context.Context,
	db *gorm.DB,
	userID uint64,
	category string,
	title string,
	body string,
	sourceType string,
	sourceID string,
	now time.Time,
) error {
	if category != "moment_moderation" {
		enabled, err := categoryEnabled(db, userID, category)
		if err != nil {
			return handleRepositoryError(err)
		}
		if !enabled {
			return nil
		}
	}
	ref := sourceID
	row := &models.Notification{
		ID:         uuid.NewString(),
		UserID:     userID,
		Category:   category,
		State:      "unread",
		Title:      title,
		Body:       body,
		SourceType: sourceType,
		SourceID:   &ref,
		CreatedAt:  now,
	}
	if err := db.Create(row).Error; err != nil {
		return handleRepositoryError(err)
	}
	// The event writer already runs inside the originating transaction. Build
	// its outbox row in that same transaction; a worker dispatches later.
	repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](db)}
	_, err := repo.CreatePendingDeliveries(ctx, []*notificationEntities.Notification{{ID: row.ID, UserID: userID}}, now)
	return err
}

func categoryEnabled(db *gorm.DB, userID uint64, category string) (bool, error) {
	var row struct {
		PointsEnabled       bool
		AnnouncementEnabled bool
	}
	err := db.Table("notification_preferences").Where("user_id = ?", userID).Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return true, nil
		}
		return false, err
	}
	if category == "points" {
		return row.PointsEnabled, nil
	}
	if category == "announcement" {
		return row.AnnouncementEnabled, nil
	}
	return true, nil
}
