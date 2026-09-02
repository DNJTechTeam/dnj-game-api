package migrations_test

import (
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMigrations_ChallengeMomentAllowsNoQRParticipationAndRejectsDuplicate(t *testing.T) {
	resetSchema(t)
	require.NoError(t, migrations.MigrateModelsWithDB(migrationSuite.DbConn))

	var userID uint64
	require.NoError(t, migrationSuite.DbConn.Raw(`
		INSERT INTO users (email, name, role, points, onboarding_complete, created_at, updated_at)
		VALUES ('challenge-moment@example.com', 'Challenge Moment', 'DEFAULT', 0, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`).Scan(&userID).Error)
	activityID := uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO activities (id, slug, name, description, kind, status, starts_at, ends_at, check_in_points, moment_points, cooldown_seconds, allows_moment, created_at, updated_at)
		VALUES (?, 'challenge-moment', 'Challenge Moment', 'Foto', 'challenge', 'active', ?, ?, 0, 50, 0, TRUE, ?, ?)
	`, activityID, now.Add(-time.Minute), now.Add(time.Minute), now, now).Error)

	assetID := uuid.NewString()
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO media_assets (id, owner_user_id, provider, bucket, staging_object_key, final_object_key, content_type, bytes, checksum_sha256, state, upload_expires_at, retention_due_at, created_at, updated_at)
		VALUES (?, ?, 's3', 'private', ?, ?, 'image/jpeg', 1, ?, 'available', ?, ?, ?, ?)
	`, assetID, userID, "staging/"+uuid.NewString(), "media/"+uuid.NewString(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", now, now.Add(90*24*time.Hour), now, now).Error)

	momentID := uuid.NewString()
	insertChallenge := migrationSuite.DbConn.Exec(`
		INSERT INTO moments (id, user_id, participation_id, activity_id, media_asset_id, origin, publication_status, moderation_status, reward_status, points_awarded, captured_at, created_at, updated_at)
		VALUES (?, ?, NULL, ?, ?, 'challenge', 'public', 'approved', 'awarded', 50, ?, ?, ?)
	`, momentID, userID, activityID, assetID, now, now, now).Error
	require.NoError(t, insertChallenge)

	insertPoint := migrationSuite.DbConn.Exec(`
		INSERT INTO point_entries (id, user_id, activity_id, activity_run_id, participation_id, moment_id, origin, reason, delta, created_at)
		VALUES (?, ?, ?, NULL, NULL, ?, 'moment', 'moment_challenge_award', 50, ?)
	`, uuid.NewString(), userID, activityID, momentID, now).Error
	require.NoError(t, insertPoint)

	secondAssetID := uuid.NewString()
	require.NoError(t, migrationSuite.DbConn.Exec(`
		INSERT INTO media_assets (id, owner_user_id, provider, bucket, staging_object_key, final_object_key, content_type, bytes, checksum_sha256, state, upload_expires_at, retention_due_at, created_at, updated_at)
		VALUES (?, ?, 's3', 'private', ?, ?, 'image/jpeg', 1, ?, 'available', ?, ?, ?, ?)
	`, secondAssetID, userID, "staging/"+uuid.NewString(), "media/"+uuid.NewString(), "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=", now, now.Add(90*24*time.Hour), now, now).Error)

	duplicate := migrationSuite.DbConn.Exec(`
		INSERT INTO moments (id, user_id, participation_id, activity_id, media_asset_id, origin, publication_status, moderation_status, reward_status, points_awarded, captured_at, created_at, updated_at)
		VALUES (?, ?, NULL, ?, ?, 'challenge', 'private', 'approved', 'denied', 0, ?, ?, ?)
	`, uuid.NewString(), userID, activityID, secondAssetID, now, now, now).Error
	require.Error(t, duplicate)
}
