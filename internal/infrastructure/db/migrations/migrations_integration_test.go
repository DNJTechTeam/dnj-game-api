package migrations_test

import (
	"os"
	"sync"
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/migrations"
	testinfra "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/di/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var migrationSuite *testinfra.Containers

func TestMain(m *testing.M) {
	migrationSuite = testinfra.ProvideUnmigratedContainers(testinfra.DbContainerName)
	code := m.Run()
	testinfra.HandleShutdown(migrationSuite)
	os.Exit(code)
}

func resetSchema(t *testing.T) {
	t.Helper()
	require.NoError(t, migrationSuite.DbConn.Exec(`DROP SCHEMA public CASCADE`).Error)
	require.NoError(t, migrationSuite.DbConn.Exec(`CREATE SCHEMA public`).Error)
}

func registeredMigrations() []migrations.Migration {
	registry := migrations.NewMigrationRegistry(migrationSuite.DbConn)
	migrations.RegisterModelMigrations(registry)
	return registry.Migrations()
}

func TestMigrations_CleanDatabaseAndTrueIdempotency(t *testing.T) {
	// given
	resetSchema(t)
	registered := registeredMigrations()

	// when
	err := migrations.MigrateModelsWithDB(migrationSuite.DbConn)
	var count int64
	countErr := migrationSuite.DbConn.Table("schema_migrations").Count(&count).Error

	// then
	require.NoError(t, err)
	require.NoError(t, countErr)
	assert.Equal(t, int64(len(registered)), count)

	// when
	for _, migration := range registered {
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" first direct replay")
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" second direct replay")
	}

	// then
	require.NoError(t, migrations.MigrateModelsWithDB(migrationSuite.DbConn))
}

func TestMigrations_UpgradeLegacyPartialSchema(t *testing.T) {
	// given
	resetSchema(t)
	require.NoError(t, migrationSuite.DbConn.Exec(`
		CREATE TABLE users (
			id bigserial PRIMARY KEY,
			email text NOT NULL,
			name text NOT NULL,
			password text,
			email_confirmed_at timestamptz,
			password_confirmed_at timestamptz,
			created_at timestamptz,
			updated_at timestamptz
		)
	`).Error)

	// when
	err := migrations.MigrateModelsWithDB(migrationSuite.DbConn)
	migrator := migrationSuite.DbConn.Migrator()

	// then
	require.NoError(t, err)
	assert.False(t, migrator.HasColumn("users", "password"))
	assert.False(t, migrator.HasColumn("users", "email_confirmed_at"))
	assert.True(t, migrator.HasColumn("users", "document"))
	assert.True(t, migrator.HasTable("subscription_webhook_verification_codes"))
}

func TestMigrations_ConcurrentRunnersSerialize(t *testing.T) {
	// given
	resetSchema(t)
	const runners = 4
	errorsByRunner := make(chan error, runners)
	var waitGroup sync.WaitGroup
	waitGroup.Add(runners)

	// when
	for range runners {
		go func() {
			defer waitGroup.Done()
			errorsByRunner <- migrations.MigrateModelsWithDB(migrationSuite.DbConn)
		}()
	}
	waitGroup.Wait()
	close(errorsByRunner)

	// then
	for err := range errorsByRunner {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, migrationSuite.DbConn.Table("schema_migrations").Count(&count).Error)
	assert.Equal(t, int64(len(registeredMigrations())), count)
}

func TestMigrations_ChecksumBackfillAndDriftDetection(t *testing.T) {
	// given
	resetSchema(t)
	require.NoError(t, migrations.MigrateModelsWithDB(migrationSuite.DbConn))
	require.NoError(t, migrationSuite.DbConn.Table("schema_migrations").Where("name = ?", "create_users_table").Update("checksum", "").Error)

	// when
	backfillErr := migrations.MigrateModelsWithDB(migrationSuite.DbConn)
	var checksum string
	lookupErr := migrationSuite.DbConn.Table("schema_migrations").Select("checksum").Where("name = ?", "create_users_table").Scan(&checksum).Error

	// then
	require.NoError(t, backfillErr)
	require.NoError(t, lookupErr)
	assert.Len(t, checksum, 64)

	// when
	require.NoError(t, migrationSuite.DbConn.Table("schema_migrations").Where("name = ?", "create_users_table").Update("checksum", "tampered").Error)
	driftErr := migrations.MigrateModelsWithDB(migrationSuite.DbConn)

	// then
	require.Error(t, driftErr)
	assert.Contains(t, driftErr.Error(), "checksum mismatch")
}
