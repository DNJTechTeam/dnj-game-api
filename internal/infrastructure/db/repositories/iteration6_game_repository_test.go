package repositories

import (
	"context"
	"errors"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIteration6GameRepository_PointBalanceAuditReturnsSafeError(t *testing.T) {
	database, mock := newMockDB(t)
	mock.ExpectQuery("SELECT users\\.id AS user_id").WillReturnError(errors.New("database unavailable"))
	repository := NewGameRepository(database)

	mismatches, err := repository.ListPointBalanceMismatches(context.Background())

	assert.Nil(t, mismatches)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIteration6GameRepository_ListOpenRunsReturnsSafeError(t *testing.T) {
	database, mock := newMockDB(t)
	mock.ExpectQuery(`SELECT .*activity_runs.*`).WillReturnError(errors.New("database unavailable"))
	repository := NewGameRepository(database)

	runs, err := repository.ListOpenRunsForManager(context.Background(), 42, true)

	assert.Nil(t, runs)
	assert.ErrorIs(t, err, appErrors.InternalError)
	require.NoError(t, mock.ExpectationsWereMet())
}
