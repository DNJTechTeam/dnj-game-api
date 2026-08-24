package repositories

import (
	"context"
	"errors"
	"testing"

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
