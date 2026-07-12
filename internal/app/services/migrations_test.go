package services

import (
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/migrations"
	"github.com/stretchr/testify/require"
)

// TestMigrations_AreIdempotent re-runs every migration on an already-migrated
// database. TestMain already applied them once via testcontainers; a second run
// must complete without error.
func TestMigrations_AreIdempotent(t *testing.T) {
	err := migrations.MigrateModels()
	require.NoError(t, err)
}
