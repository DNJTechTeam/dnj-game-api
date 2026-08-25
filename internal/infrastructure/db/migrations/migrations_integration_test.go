package migrations_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

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

func TestMigrations_MediaMomentsUpgradePreservesPriorDataAndEnforcesHistory(t *testing.T) {
	resetSchema(t)
	registered := registeredMigrations()
	for _, migration := range registered {
		if migration.Name == "expand_media_moments_v2" {
			break
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name)
	}
	var userID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`
		INSERT INTO users (email,name,role,points,onboarding_complete,created_at,updated_at)
		VALUES ('media-upgrade@example.com','Media Upgrade','DEFAULT',0,TRUE,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
		RETURNING id
	`).Scan(&userID).Error)
	legacyKey := uuid.NewString()
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO admin_operations
		(id,actor_user_id,idempotency_key,operation,entity_type,entity_ref,request_hash,http_status,response,created_at)
		VALUES (?, ?, ?, 'admin.user.role.update', 'user', ?, ?, 200, '{}'::JSONB, CURRENT_TIMESTAMP)
	`, uuid.NewString(), userID, legacyKey, fmt.Sprint(userID), "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef").Error)

	for _, migration := range registered {
		if migration.Name != "expand_media_moments_v2" &&
			migration.Name != "backfill_global_idempotency_registry" &&
			migration.Name != "contract_media_moments_v2" {
			continue
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" first replay")
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" second replay")
	}

	assetID := uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO media_assets
		(id,owner_user_id,provider,bucket,staging_object_key,final_object_key,content_type,bytes,checksum_sha256,state,upload_expires_at,retention_due_at,created_at,updated_at)
		VALUES (?, ?, 's3', 'private', ?, ?, 'image/jpeg', 1, ?, 'available', ?, ?, ?, ?)
	`, assetID, userID, "staging/"+uuid.NewString(), "media/"+uuid.NewString(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", now, now.Add(90*24*time.Hour), now, now).Error)
	momentID := uuid.NewString()
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO moments
		(id,user_id,media_asset_id,origin,publication_status,moderation_status,reward_status,points_awarded,captured_at,created_at,updated_at)
		VALUES (?, ?, ?, 'free', 'public', 'approved', 'not_applicable', 0, ?, ?, ?)
	`, momentID, userID, assetID, now, now, now).Error)
	duplicateAssetErr := migrationSuite.DbConn.Exec(`
		INSERT INTO moments
		(id,user_id,media_asset_id,origin,publication_status,moderation_status,reward_status,points_awarded,captured_at,created_at,updated_at)
		VALUES (?, ?, ?, 'free', 'private', 'approved', 'not_applicable', 0, ?, ?, ?)
	`, uuid.NewString(), userID, assetID, now, now, now).Error
	badFreeRewardErr := migrationSuite.DbConn.Exec(`
		UPDATE moments SET points_awarded=1 WHERE id=?
	`, momentID).Error
	require.NoError(t, migrationSuite.DbConn.Exec(`UPDATE users SET deleted_at=? WHERE id=?`, now, userID).Error)

	var history int64
	require.NoError(t, migrationSuite.DbConn.Table("moments").Where("id=?", momentID).Count(&history).Error)
	var legacyRegistry int64
	require.NoError(t, migrationSuite.DbConn.Table("idempotency_operations").
		Where("actor_user_id=? AND idempotency_key=?", userID, legacyKey).Count(&legacyRegistry).Error)
	require.Error(t, duplicateAssetErr)
	require.Error(t, badFreeRewardErr)
	assert.EqualValues(t, 1, history)
	assert.EqualValues(t, 1, legacyRegistry)
	for _, table := range []string{
		"media_assets", "moments", "moment_likes", "idempotency_operations",
		"media_processing_claims", "media_cleanup_jobs", "moment_moderation_decisions",
	} {
		assert.True(t, migrationSuite.DbConn.Migrator().HasTable(table), table)
	}
	assert.False(t, migrationSuite.DbConn.Migrator().HasTable("gallery"))
	assert.False(t, migrationSuite.DbConn.Migrator().HasTable("events"))
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

func TestMigrations_Iteration5UpgradePreservesIteration4AndParticipantConstraints(t *testing.T) {
	// given
	resetSchema(t)
	registered := registeredMigrations()
	for _, migration := range registered {
		if migration.Name == "expand_iteration5_agenda_content" {
			break
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name)
	}
	var userID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`
		INSERT INTO users (email, name, role, onboarding_complete, created_at, updated_at)
		VALUES ('iteration5-upgrade@example.com', 'Iteration 5 Upgrade', 'DEFAULT', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`).Scan(&userID).Error)
	activityID := "99999999-9999-4999-8999-999999999999"
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, slug, name, kind, status, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES (?, 'iteration5-upgrade', 'Iteration 5 Upgrade', 'live', 'active', 0, 0, 0, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, activityID).Error)

	// when
	for _, migration := range registered {
		if migration.Name != "expand_iteration5_agenda_content" && migration.Name != "backfill_iteration5_agenda_content" && migration.Name != "contract_iteration5_agenda_content" {
			continue
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" first replay")
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" second replay")
	}
	insertFavoriteErr := migrationSuite.DbConn.Exec(`INSERT INTO user_favorites (user_id, activity_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, userID, activityID).Error
	duplicateFavoriteErr := migrationSuite.DbConn.Exec(`INSERT INTO user_favorites (user_id, activity_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, userID, activityID).Error
	key := uuid.NewString()
	insertOperationErr := migrationSuite.DbConn.Exec(`INSERT INTO participant_operations (id, actor_user_id, idempotency_key, operation, activity_id, intent_hash, http_status, created_at) VALUES (?, ?, ?, 'favorite.delete', ?, ?, 204, CURRENT_TIMESTAMP)`, uuid.NewString(), userID, key, uuid.NewString(), "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef").Error
	duplicateOperationErr := migrationSuite.DbConn.Exec(`INSERT INTO participant_operations (id, actor_user_id, idempotency_key, operation, activity_id, intent_hash, http_status, created_at) VALUES (?, ?, ?, 'favorite.delete', ?, ?, 204, CURRENT_TIMESTAMP)`, uuid.NewString(), userID, key, uuid.NewString(), "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789").Error
	badStatusErr := migrationSuite.DbConn.Exec(`INSERT INTO participant_operations (id, actor_user_id, idempotency_key, operation, activity_id, intent_hash, http_status, created_at) VALUES (?, ?, ?, 'favorite.put', ?, ?, 200, CURRENT_TIMESTAMP)`, uuid.NewString(), userID, uuid.NewString(), activityID, "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210").Error
	var preserved int64
	require.NoError(t, migrationSuite.DbConn.Table("activities").Where("id = ?", activityID).Count(&preserved).Error)
	migrator := migrationSuite.DbConn.Migrator()

	// then
	require.NoError(t, insertFavoriteErr)
	require.Error(t, duplicateFavoriteErr)
	require.NoError(t, insertOperationErr)
	require.Error(t, duplicateOperationErr)
	require.Error(t, badStatusErr)
	assert.Equal(t, int64(1), preserved)
	assert.True(t, migrator.HasTable("user_favorites"))
	assert.True(t, migrator.HasTable("participant_operations"))
	assert.False(t, migrator.HasColumn("user_favorites", "event_id"))
	assert.False(t, migrator.HasColumn("participant_operations", "event_id"))
}

func TestMigrations_Iteration6UpgradePreservesPriorDataAndEnforcesGameInvariants(t *testing.T) {
	// given
	resetSchema(t)
	registered := registeredMigrations()
	for _, migration := range registered {
		if migration.Name == "expand_iteration6_games_runs_scoring" {
			break
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name)
	}
	var managerID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`
		INSERT INTO users (email, name, role, points, onboarding_complete, created_at, updated_at)
		VALUES ('iteration6-manager@example.com', 'Iteration 6 Manager', 'EVENT_MANAGER', 0, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`).Scan(&managerID).Error)
	var participantID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`
		INSERT INTO users (email, name, role, points, onboarding_complete, created_at, updated_at)
		VALUES ('iteration6-participant@example.com', 'Iteration 6 Participant', 'DEFAULT', 37, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`).Scan(&participantID).Error)
	activityID := uuid.NewString()
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, slug, name, kind, status, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES (?, 'iteration6-game', 'Iteration 6 Game', 'competitive', 'active', 0, 0, 0, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, activityID).Error)

	// when
	for _, migration := range registered {
		if migration.Name != "expand_iteration6_games_runs_scoring" && migration.Name != "backfill_iteration6_games_runs_scoring" && migration.Name != "contract_iteration6_games_runs_scoring" {
			continue
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" first replay")
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name+" second replay")
	}
	runID := uuid.NewString()
	insertRunErr := migrationSuite.DbConn.Exec(`INSERT INTO activity_runs (id, activity_id, started_by, status, point_rules, created_at, updated_at) VALUES (?, ?, ?, 'draft', CAST('{"first":50,"second":30,"third":20,"participation":10}' AS JSONB), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, runID, activityID, managerID).Error
	duplicateOpenErr := migrationSuite.DbConn.Exec(`INSERT INTO activity_runs (id, activity_id, started_by, status, point_rules, created_at, updated_at) VALUES (?, ?, ?, 'active', CAST('{"first":50,"second":30,"third":20,"participation":10}' AS JSONB), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, uuid.NewString(), activityID, managerID).Error
	negativeBalanceErr := migrationSuite.DbConn.Exec(`UPDATE users SET points = -1 WHERE id = ?`, participantID).Error
	terminalWithoutEndErr := migrationSuite.DbConn.Exec(`UPDATE activity_runs SET status = 'completed' WHERE id = ?`, runID).Error
	pointMutationErr := migrationSuite.DbConn.Exec(`UPDATE point_entries SET delta = delta + 1 WHERE user_id = ?`, participantID).Error
	pointDeleteErr := migrationSuite.DbConn.Exec(`DELETE FROM point_entries WHERE user_id = ?`, participantID).Error
	var legacyEntries int64
	require.NoError(t, migrationSuite.DbConn.Table("point_entries").Where("user_id = ? AND origin = 'legacy_balance' AND delta = 37", participantID).Count(&legacyEntries).Error)
	var ledgerTotal int64
	require.NoError(t, migrationSuite.DbConn.Table("point_entries").Where("user_id = ?", participantID).Select("COALESCE(SUM(delta), 0)").Scan(&ledgerTotal).Error)
	var preserved int64
	require.NoError(t, migrationSuite.DbConn.Table("activities").Where("id = ?", activityID).Count(&preserved).Error)
	migrator := migrationSuite.DbConn.Migrator()

	// then
	require.NoError(t, insertRunErr)
	require.Error(t, duplicateOpenErr)
	require.Error(t, negativeBalanceErr)
	require.Error(t, terminalWithoutEndErr)
	require.Error(t, pointMutationErr)
	require.Error(t, pointDeleteErr)
	assert.Equal(t, int64(1), legacyEntries)
	assert.Equal(t, int64(37), ledgerTotal)
	assert.Equal(t, int64(1), preserved)
	for _, table := range []string{"activity_runs", "activity_run_qr_codes", "participations", "activity_run_participants", "point_entries", "manager_operations"} {
		assert.True(t, migrator.HasTable(table), table)
		assert.False(t, migrator.HasColumn(table, "event_id"), table)
	}
	assert.True(t, migrator.HasColumn("participant_operations", "result_ref"))
	assert.True(t, migrator.HasColumn("participant_operations", "result_points"))
	assert.False(t, migrator.HasTable("events"))
}

func TestMigrations_Iteration8UpgradePreservesPriorDataAndEnforcesNotificationInvariants(t *testing.T) {
	// given
	resetSchema(t)
	registered := registeredMigrations()
	for _, migration := range registered {
		if migration.Name == "create_notifications_v1" {
			break
		}
		require.NoError(t, migration.Up(migrationSuite.DbConn), migration.Name)
	}
	var userID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`
		INSERT INTO users (email, name, role, points, onboarding_complete, created_at, updated_at)
		VALUES ('iteration8-user@example.com', 'Iteration 8 User', 'DEFAULT', 0, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`).Scan(&userID).Error)
	activityID := uuid.NewString()
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, slug, name, kind, status, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES (?, 'iteration8-activity', 'Iteration 8 Activity', 'competitive', 'active', 0, 0, 0, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, activityID).Error)

	// when: the migration runs twice — a genuine idempotency replay
	var notificationsMigration migrations.Migration
	for _, migration := range registered {
		if migration.Name == "create_notifications_v1" {
			notificationsMigration = migration
			break
		}
	}
	require.NoError(t, notificationsMigration.Up(migrationSuite.DbConn), "first apply")
	require.NoError(t, notificationsMigration.Up(migrationSuite.DbConn), "second apply (replay)")

	migrator := migrationSuite.DbConn.Migrator()
	assert.True(t, migrator.HasTable("notifications"))
	assert.True(t, migrator.HasTable("notification_preferences"))

	badUserErr := migrationSuite.DbConn.Exec(`
		INSERT INTO notifications (id, user_id, category, state, title, body, source_type, created_at)
		VALUES (?, 999999, 'points', 'unread', 'T', 'B', 'test', CURRENT_TIMESTAMP)
	`, uuid.NewString()).Error
	badCategoryErr := migrationSuite.DbConn.Exec(`
		INSERT INTO notifications (id, user_id, category, state, title, body, source_type, created_at)
		VALUES (?, ?, 'not-a-category', 'unread', 'T', 'B', 'test', CURRENT_TIMESTAMP)
	`, uuid.NewString(), userID).Error
	badStateErr := migrationSuite.DbConn.Exec(`
		INSERT INTO notifications (id, user_id, category, state, title, body, source_type, created_at)
		VALUES (?, ?, 'points', 'not-a-state', 'T', 'B', 'test', CURRENT_TIMESTAMP)
	`, uuid.NewString(), userID).Error
	okErr := migrationSuite.DbConn.Exec(`
		INSERT INTO notifications (id, user_id, category, state, title, body, source_type, created_at)
		VALUES (?, ?, 'moment_moderation', 'unread', 'T', 'B', 'test', CURRENT_TIMESTAMP)
	`, uuid.NewString(), userID).Error
	badPreferenceUserErr := migrationSuite.DbConn.Exec(`
		INSERT INTO notification_preferences (user_id, points_enabled, announcement_enabled, updated_at)
		VALUES (999999, TRUE, TRUE, CURRENT_TIMESTAMP)
	`).Error
	var preserved int64
	require.NoError(t, migrationSuite.DbConn.Table("activities").Where("id = ?", activityID).Count(&preserved).Error)
	var preservedUser int64
	require.NoError(t, migrationSuite.DbConn.Table("users").Where("id = ?", userID).Count(&preservedUser).Error)

	// then
	require.Error(t, badUserErr)
	require.Error(t, badCategoryErr)
	require.Error(t, badStateErr)
	require.NoError(t, okErr)
	require.Error(t, badPreferenceUserErr)
	assert.Equal(t, int64(1), preserved)
	assert.Equal(t, int64(1), preservedUser)
}
