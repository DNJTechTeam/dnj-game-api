package repositories

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIteration6GameRepository_PointBalanceAuditReturnsSafeError(t *testing.T) {
	database, mock := newMockDB(t)
	mock.ExpectQuery("SELECT users\\.id AS user_id").WillReturnError(errors.New("database unavailable"))
	repository := &GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](database)}

	mismatches, err := repository.ListPointBalanceMismatches(context.Background())

	assert.Nil(t, mismatches)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIteration6GameRepository_ListOpenRunsReturnsSafeError(t *testing.T) {
	database, mock := newMockDB(t)
	mock.ExpectQuery(`SELECT .*activity_runs.*`).WillReturnError(errors.New("database unavailable"))
	repository := &GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](database)}

	runs, err := repository.ListOpenRunsForManager(context.Background(), 42, true)

	assert.Nil(t, runs)
	assert.ErrorIs(t, err, appErrors.InternalError)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIteration6GameRepository_ListOpenRunsReturnsEmptyList(t *testing.T) {
	database, mock := newMockDB(t)
	mock.ExpectQuery(`SELECT .*activity_runs.*`).WillReturnRows(sqlmock.NewRows([]string{"id", "activity_id", "started_by", "status", "point_rules", "started_at", "ended_at", "created_at", "updated_at"}))
	repository := &GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](database)}

	runs, err := repository.ListOpenRunsForManager(context.Background(), 42, true)

	require.NoError(t, err)
	assert.Empty(t, runs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIteration6GameRepository_FindOpenRunForManagerReturnsSafeError(t *testing.T) {
	database, mock := newMockDB(t)
	mock.ExpectQuery(`SELECT .*activity_runs.*`).WillReturnError(errors.New("database unavailable"))
	repository := &GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](database)}

	run, err := repository.FindOpenRunForManager(context.Background(), 42, true)

	assert.Nil(t, run)
	assert.ErrorIs(t, err, appErrors.InternalError)
	require.NoError(t, mock.ExpectationsWereMet())
}
