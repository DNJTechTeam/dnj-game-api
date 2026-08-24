package migrations_test

import (
	"os"
	"sync"
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/migrations"
	testinfra "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/di/test"
	"github.com/google/uuid"
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

func TestMigrations_Iteration3BackfillPreservesLegacyGroupID(t *testing.T) {
	// given
	resetSchema(t)
	require.NoError(t, migrations.MigrateModelsWithDB(migrationSuite.DbConn))
	var groupID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`INSERT INTO groups (name) VALUES (?) RETURNING id`, "Legacy Group").Scan(&groupID).Error)
	var userID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`INSERT INTO users (email, name, role, group_id, points, onboarding_complete, created_at, updated_at) VALUES (?, ?, ?, ?, 0, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING id`, "legacy-iteration3@example.com", "Legacy User", "DEFAULT", groupID).Scan(&userID).Error)
	var backfill migrations.Migration
	for _, migration := range registeredMigrations() {
		if migration.Name == "backfill_iteration3_group_memberships" {
			backfill = migration
			break
		}
	}
	require.NotNil(t, backfill.Up)

	// when
	firstErr := backfill.Up(migrationSuite.DbConn)
	secondErr := backfill.Up(migrationSuite.DbConn)
	var membershipCount int64
	countErr := migrationSuite.DbConn.Table("group_memberships").Where("user_id = ? AND group_id = ?", userID, groupID).Count(&membershipCount).Error
	var preservedGroupID uint64
	userErr := migrationSuite.DbConn.Table("users").Select("group_id").Where("id = ?", userID).Scan(&preservedGroupID).Error

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, countErr)
	require.NoError(t, userErr)
	assert.Equal(t, int64(1), membershipCount)
	assert.Equal(t, groupID, preservedGroupID)
}

func TestMigrations_Iteration4PartialUpgradePreservesRowsAndRejectsMultiEventColumns(t *testing.T) {
	// given
	resetSchema(t)
	require.NoError(t, migrationSuite.DbConn.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email TEXT NOT NULL,
			name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'DEFAULT',
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		);
		CREATE TABLE spaces (
			id UUID PRIMARY KEY,
			slug VARCHAR(120),
			name VARCHAR(200)
		);
		CREATE TABLE activities (
			id UUID PRIMARY KEY,
			slug VARCHAR(120),
			name VARCHAR(200),
			kind VARCHAR(32)
		);
		INSERT INTO spaces (id, slug, name) VALUES ('11111111-1111-4111-8111-111111111111', 'capela', 'Capela');
		INSERT INTO activities (id, slug, name, kind) VALUES ('22222222-2222-4222-8222-222222222222', 'radicalidade', 'Radicalidade', 'competitive')
	`).Error)

	// when
	err := migrations.MigrateModelsWithDB(migrationSuite.DbConn)
	migrator := migrationSuite.DbConn.Migrator()
	var spaces int64
	var activities int64
	require.NoError(t, migrationSuite.DbConn.Table("spaces").Count(&spaces).Error)
	require.NoError(t, migrationSuite.DbConn.Table("activities").Count(&activities).Error)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), spaces)
	assert.Equal(t, int64(1), activities)
	assert.True(t, migrator.HasTable("activity_manager_assignments"))
	assert.True(t, migrator.HasTable("operation_audit"))
	assert.False(t, migrator.HasTable("events"))
	assert.False(t, migrator.HasColumn("spaces", "event_id"))
	assert.False(t, migrator.HasColumn("activities", "event_id"))
}

func TestMigrations_Iteration4ActivityConfigurationConstraints(t *testing.T) {
	// given
	resetSchema(t)
	require.NoError(t, migrations.MigrateModelsWithDB(migrationSuite.DbConn))
	validID := "33333333-3333-4333-8333-333333333333"

	// when
	validErr := migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, space_id, slug, name, description, kind, status, starts_at, ends_at, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES (?, NULL, 'photo-challenge', 'Photo challenge', 'Description', 'challenge', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '10 minutes', 10, 30, 60, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, validID).Error
	negativePointsErr := migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, slug, name, kind, status, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES ('44444444-4444-4444-8444-444444444444', 'negative-points', 'Negative', 'live', 'draft', -1, 0, 0, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`).Error
	invalidWindowErr := migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, slug, name, kind, status, starts_at, ends_at, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES ('55555555-5555-4555-8555-555555555555', 'invalid-window', 'Invalid window', 'live', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP - INTERVAL '1 minute', 0, 0, 0, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`).Error
	scheduleMomentErr := migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, slug, name, kind, status, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES ('66666666-6666-4666-8666-666666666666', 'schedule-moment', 'Schedule Moment', 'schedule', 'draft', 0, 0, 0, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`).Error

	// then
	require.NoError(t, validErr)
	require.Error(t, negativePointsErr)
	require.Error(t, invalidWindowErr)
	require.Error(t, scheduleMomentErr)
	var spaceID *string
	require.NoError(t, migrationSuite.DbConn.Table("activities").Select("space_id").Where("id = ?", validID).Scan(&spaceID).Error)
	assert.Nil(t, spaceID)
}

func TestMigrations_AdminEnablerUpgradesIteration4RowsAndReplays(t *testing.T) {
	// given
	resetSchema(t)
	registered := registeredMigrations()
	for _, migration := range registered {
		if migration.Name == "expand_iteration4_admin_enabler" {
			break
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name)
	}
	var actorID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`
		INSERT INTO users (email, name, role, onboarding_complete, created_at, updated_at)
		VALUES ('upgrade-admin@example.com', 'Upgrade Admin', 'ADMIN', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`).Scan(&actorID).Error)
	activityID := "77777777-7777-4777-8777-777777777777"
	auditID := "88888888-8888-4888-8888-888888888888"
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, slug, name, kind, status, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES (?, 'upgrade-activity', 'Upgrade Activity', 'live', 'draft', 0, 0, 0, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, activityID).Error)
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO operation_audit (id, actor_user_id, action, entity_type, entity_id, metadata, idempotency_key, created_at)
		VALUES (?, ?, 'activity.start', 'activity', ?, CAST('{"fromStatus":"draft","toStatus":"active"}' AS JSONB), ?, CURRENT_TIMESTAMP)
	`, auditID, actorID, activityID, uuid.NewString()).Error)

	// when
	for _, migration := range registered {
		if migration.Name != "expand_iteration4_admin_enabler" && migration.Name != "backfill_iteration4_admin_enabler" && migration.Name != "contract_iteration4_admin_enabler" {
			continue
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" first replay")
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" second replay")
	}
	var preservedActivities int64
	var preservedAudits int64
	var entityReference string
	require.NoError(t, migrationSuite.DbConn.Table("activities").Where("id = ?", activityID).Count(&preservedActivities).Error)
	require.NoError(t, migrationSuite.DbConn.Table("operation_audit").Where("id = ?", auditID).Count(&preservedAudits).Error)
	require.NoError(t, migrationSuite.DbConn.Table("operation_audit").Select("entity_reference").Where("id = ?", auditID).Scan(&entityReference).Error)
	migrator := migrationSuite.DbConn.Migrator()

	// then
	assert.Equal(t, int64(1), preservedActivities)
	assert.Equal(t, int64(1), preservedAudits)
	assert.Equal(t, activityID, entityReference)
	assert.True(t, migrator.HasTable("admin_operations"))
	assert.True(t, migrator.HasColumn("operation_audit", "entity_reference"))
	assert.False(t, migrator.HasTable("events"))
	assert.False(t, migrator.HasColumn("activities", "event_id"))
}
