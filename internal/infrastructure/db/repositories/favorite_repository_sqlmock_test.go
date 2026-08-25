package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
)

// TestIteration5_FavoriteRepositorySQLFailures exercises favorite_repository.go's error
// branches that require a genuinely broken connection: the visibility listing query, a generic
// write failure on Create/Delete, a generic operation lookup failure, and a generic failure
// reserving the (now-unified) global idempotency key during CreateOperation.
func TestIteration5_FavoriteRepositorySQLFailures(t *testing.T) {
	t.Run("ListVisible: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &FavoriteRepository{BaseRepository: NewBaseRepository[models.UserFavorite](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.ListVisible(context.Background(), 1, time.Now(), 0)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("Create: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &FavoriteRepository{BaseRepository: NewBaseRepository[models.UserFavorite](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		_, err := repo.Create(context.Background(), &entities.Favorite{UserID: 1, ActivityID: "activity-1"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("Delete: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &FavoriteRepository{BaseRepository: NewBaseRepository[models.UserFavorite](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		_, err := repo.Delete(context.Background(), 1, "activity-1")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("FindOperation: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &FavoriteRepository{BaseRepository: NewBaseRepository[models.UserFavorite](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.FindOperation(context.Background(), 1, "key")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CreateOperation: a generic idempotency reservation failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &FavoriteRepository{BaseRepository: NewBaseRepository[models.UserFavorite](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		_, err := repo.CreateOperation(context.Background(), &entities.ParticipantOperation{ID: "op-1", CreatedAt: time.Now()})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
