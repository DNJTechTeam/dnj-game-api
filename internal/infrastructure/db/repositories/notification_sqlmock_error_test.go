package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
)

// TestNotifications_RepositorySQLFailures exercises notification_repository.go's error
// branches that require a genuinely broken connection, not reachable through the
// real-Postgres integration suite.
func TestNotifications_RepositorySQLFailures(t *testing.T) {
	t.Run("FindPreferences: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.FindPreferences(context.Background(), 1)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("UpsertPreferences: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		_, err := repo.UpsertPreferences(context.Background(), &entities.Preferences{UserID: 1})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("List: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.List(context.Background(), 1, 0)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CountUnread: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.CountUnread(context.Background(), 1)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("FindByIDAndUser: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.FindByIDAndUser(context.Background(), "n-1", 1)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("MarkRead: an update failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		_, err := repo.MarkRead(context.Background(), "n-1", 1, time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("ResolveAnnouncementRecipients: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.ResolveAnnouncementRecipients(context.Background(), nil)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CreateBroadcast: an empty slice is a no-op", func(t *testing.T) {
		gormDB, _ := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		assert.NoError(t, repo.CreateBroadcast(context.Background(), nil))
	})

	t.Run("CreateBroadcast: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		err := repo.CreateBroadcast(context.Background(), []*entities.Notification{{ID: "n-1", UserID: 1}})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("FindOperation: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.FindOperation(context.Background(), 1, "key-1")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CreateOperation: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &NotificationRepository{BaseRepository: NewBaseRepository[models.Notification](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		err := repo.CreateOperation(context.Background(), &entities.Operation{ID: "op-1"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
