package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testChecksum() string {
	sum := sha256.Sum256([]byte(uuid.NewString()))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func seedMediaAsset(t *testing.T, ctx context.Context, repo *MediaRepository, owner uint64, state string, uploadExpiresAt time.Time) *entities.Asset {
	t.Helper()
	id := uuid.NewString()
	asset := &entities.Asset{
		ID:               id,
		OwnerUserID:      owner,
		Provider:         "s3",
		Bucket:           "dnj-media-local",
		StagingObjectKey: "staging/" + uuid.NewString(),
		FinalObjectKey:   "final/" + uuid.NewString(),
		ContentType:      "image/jpeg",
		Bytes:            1024,
		ChecksumSHA256:   testChecksum(),
		State:            entities.AssetState(state),
		UploadExpiresAt:  uploadExpiresAt,
		RetentionDueAt:   uploadExpiresAt.Add(90 * 24 * time.Hour),
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	require.NoError(t, repo.CreateAsset(ctx, asset))
	return asset
}

// TestMediaMoments_CleanupRepositoryLifecycle exercises the media cleanup/retention
// repository methods that the internal worker relies on: expiring stale pending
// uploads, claiming and completing cleanup jobs with a durable lease, retrying a
// failed attempt, and reporting aggregate worker metrics — none of which are
// reachable through the HTTP-facing service tests, since the worker talks to the
// repository directly.
func TestMediaMoments_CleanupRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.MediaCleanupJob{})
	TestSuite.TruncateTable(t, &models.MediaProcessingClaim{})
	TestSuite.TruncateTable(t, &models.Moment{})
	TestSuite.TruncateTable(t, &models.MediaAsset{})
	TestSuite.TruncateTable(t, &models.User{})

	repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](TestSuite.DbConn)}
	owner := seedUser(t, ctx, "media-cleanup-owner@example.com")
	now := time.Now().UTC()

	// ── ExpirePendingAssets ───────────────────────────────────────────────
	stale := seedMediaAsset(t, ctx, repo, owner.ID, string(entities.AssetPendingUpload), now.Add(-time.Hour))
	fresh := seedMediaAsset(t, ctx, repo, owner.ID, string(entities.AssetPendingUpload), now.Add(time.Hour))

	expired, err := repo.ExpirePendingAssets(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, stale.ID, expired[0].ID)
	assert.Equal(t, entities.AssetFailed, expired[0].State)
	assert.NotNil(t, expired[0].FailedAt)

	stillPending, err := repo.FindAsset(ctx, fresh.ID, false)
	require.NoError(t, err)
	assert.Equal(t, entities.AssetPendingUpload, stillPending.State)

	// Re-running is idempotent: nothing left to expire for the same asset.
	expiredAgain, err := repo.ExpirePendingAssets(ctx, now, 10)
	require.NoError(t, err)
	assert.Empty(t, expiredAgain)

	// ── AcquireProcessingClaim / UpdateProcessingClaim ────────────────────
	claimAsset := seedMediaAsset(t, ctx, repo, owner.ID, string(entities.AssetProcessing), now.Add(time.Hour))
	token1, token2, token3, foreignToken := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	opKey1, opKey2 := uuid.NewString(), uuid.NewString()
	claim, acquired, err := repo.AcquireProcessingClaim(ctx, claimAsset.ID, token1, opKey1, now, now.Add(5*time.Minute))
	require.NoError(t, err)
	assert.True(t, acquired)
	require.NotNil(t, claim)
	assert.Equal(t, claimAsset.ID, claim.MediaAssetID)

	// A concurrent attempt while the lease is still active must not acquire.
	_, acquiredAgain, err := repo.AcquireProcessingClaim(ctx, claimAsset.ID, token2, opKey2, now, now.Add(5*time.Minute))
	require.NoError(t, err)
	assert.False(t, acquiredAgain)

	claim.Stage = "finalized"
	finalVersion := "v-final-1"
	claim.FinalVersionID = &finalVersion
	claim.LeaseExpiresAt = now.Add(10 * time.Minute)
	claim.UpdatedAt = now
	require.NoError(t, repo.UpdateProcessingClaim(ctx, claim))

	// Updating with a stale/foreign claim token is a conflict, not a silent no-op.
	err = repo.UpdateProcessingClaim(ctx, &entities.ProcessingClaim{MediaAssetID: claimAsset.ID, ClaimToken: foreignToken, UpdatedAt: now})
	assert.ErrorIs(t, err, appErrors.ErrConflict)

	// After the lease expires, another worker can resume the same claim.
	claimResumed, resumedAcquired, err := repo.AcquireProcessingClaim(ctx, claimAsset.ID, token3, opKey1, now.Add(20*time.Minute), now.Add(25*time.Minute))
	require.NoError(t, err)
	assert.True(t, resumedAcquired)
	assert.Equal(t, 2, claimResumed.AttemptCount)

	// ── CreateCleanupJob / ClaimCleanupJobs / RetryCleanupJob / CompleteCleanupJob ──
	jobAsset := seedMediaAsset(t, ctx, repo, owner.ID, string(entities.AssetDeleted), now.Add(time.Hour))
	jobID := uuid.NewString()
	created, err := repo.CreateCleanupJob(ctx, &entities.CleanupJob{
		ID:            jobID,
		MediaAssetID:  jobAsset.ID,
		Kind:          "delete_photo",
		State:         "pending",
		DueAt:         now.Add(-time.Minute),
		AttemptCount:  0,
		MaxAttempts:   3,
		NextAttemptAt: now.Add(-time.Minute),
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	require.NoError(t, err)
	assert.True(t, created)

	// Duplicate insert for the same job id is a durable no-op (idempotent enqueue).
	createdAgain, err := repo.CreateCleanupJob(ctx, &entities.CleanupJob{
		ID: jobID, MediaAssetID: jobAsset.ID, Kind: "delete_photo", State: "pending",
		DueAt: now, AttemptCount: 0, MaxAttempts: 3, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	assert.False(t, createdAgain)

	claimed, err := repo.ClaimCleanupJobs(ctx, now, 5*time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, jobID, claimed[0].ID)
	assert.Equal(t, "processing", claimed[0].State)
	require.NotNil(t, claimed[0].ClaimToken)

	// While leased, a concurrent claim attempt must not pick up the same job.
	claimedAgain, err := repo.ClaimCleanupJobs(ctx, now, 5*time.Minute, 10)
	require.NoError(t, err)
	assert.Empty(t, claimedAgain)

	// Completing with the wrong token is rejected.
	err = repo.CompleteCleanupJob(ctx, jobID, uuid.NewString(), now)
	assert.ErrorIs(t, err, appErrors.ErrConflict)

	// Retrying with the wrong token is rejected, not silently accepted.
	err = repo.RetryCleanupJob(ctx, jobID, uuid.NewString(), now.Add(time.Minute), "provider_unavailable", now)
	assert.ErrorIs(t, err, appErrors.ErrConflict)

	// A retry with the correct token records the failure and reschedules.
	err = repo.RetryCleanupJob(ctx, jobID, *claimed[0].ClaimToken, now.Add(time.Minute), "provider_unavailable", now)
	require.NoError(t, err)

	// The job is claimable again once its lease clears (RetryCleanupJob nulls it).
	reclaimed, err := repo.ClaimCleanupJobs(ctx, now.Add(2*time.Minute), 5*time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.NotNil(t, reclaimed[0].ClaimToken)

	require.NoError(t, repo.CompleteCleanupJob(ctx, jobID, *reclaimed[0].ClaimToken, now.Add(3*time.Minute)))

	// Completing an already-completed job a second time is a safe conflict, not a re-effect.
	err = repo.CompleteCleanupJob(ctx, jobID, *reclaimed[0].ClaimToken, now.Add(3*time.Minute))
	assert.ErrorIs(t, err, appErrors.ErrConflict)

	// ── Metrics ────────────────────────────────────────────────────────────
	metrics, err := repo.Metrics(ctx, now.Add(3*time.Minute))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, metrics.Retries, int64(1))
	assert.GreaterOrEqual(t, metrics.Expired, int64(0))

	// ── RetryCleanupJob exhausting its attempt budget moves to "failed" ─────
	exhaustedAsset := seedMediaAsset(t, ctx, repo, owner.ID, string(entities.AssetDeleted), now.Add(time.Hour))
	exhaustedJobID := uuid.NewString()
	_, err = repo.CreateCleanupJob(ctx, &entities.CleanupJob{
		ID: exhaustedJobID, MediaAssetID: exhaustedAsset.ID, Kind: "delete_photo", State: "pending",
		DueAt: now.Add(-time.Minute), AttemptCount: 0, MaxAttempts: 1, NextAttemptAt: now.Add(-time.Minute),
		CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	exhaustedClaim, err := repo.ClaimCleanupJobs(ctx, now, 5*time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, exhaustedClaim, 1)
	require.NoError(t, repo.RetryCleanupJob(ctx, exhaustedJobID, *exhaustedClaim[0].ClaimToken, now.Add(time.Minute), "provider_unavailable", now))
	failedMetrics, err := repo.Metrics(ctx, now.Add(time.Minute))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, failedMetrics.Failed, int64(1))

	// ── CompleteOperation on an operation that is not "processing" is a conflict ──
	opRow := &entities.Operation{
		ID: uuid.NewString(), ActorUserID: owner.ID, IdempotencyKey: uuid.NewString(),
		Operation: "media.upload.complete", IntentHash: "hash", State: "completed",
		ResponseSnapshot: []byte(`{}`), HTTPStatus: 200, CreatedAt: now,
	}
	require.NoError(t, repo.CreateOperation(ctx, opRow))
	err = repo.CompleteOperation(ctx, opRow.ID, 200, nil, nil, nil, []byte(`{}`), now)
	assert.ErrorIs(t, err, appErrors.ErrConflict)
}
