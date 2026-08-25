package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
)

// TestAdminInstallation_OperationRepositorySQLFailures exercises admin_operation_repository.go's
// error branches that require a genuinely broken connection: a generic lookup failure, and a
// generic failure reserving the (now-unified) global idempotency key during Create.
func TestAdminInstallation_OperationRepositorySQLFailures(t *testing.T) {
	t.Run("FindByActorAndIdempotencyKey: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &AdminOperationRepository{BaseRepository: NewBaseRepository[models.AdminOperation](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.FindByActorAndIdempotencyKey(context.Background(), 1, "key")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("Create: a generic idempotency reservation failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &AdminOperationRepository{BaseRepository: NewBaseRepository[models.AdminOperation](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		_, err := repo.Create(context.Background(), &entities.AdminOperation{ID: "op-1", CreatedAt: time.Now()})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
