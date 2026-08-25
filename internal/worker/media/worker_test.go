package media

import (
	"context"
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var workerNow = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

func TestMediaWorker_ExpiresUploadsDeletesVersionsAndPublishesMetrics(t *testing.T) {
	repository := mocks.NewMockMediaRepository(t)
	storage := mocks.NewMockMediaStorage(t)
	claim := "claim-token"
	expired := mediaEntities.Asset{ID: "expired", Bucket: "private", StagingObjectKey: "staging/a"}
	retained := mediaEntities.Asset{
		ID:               "retained",
		Bucket:           "private",
		StagingObjectKey: "staging/b",
		FinalObjectKey:   "media/b.jpg",
		State:            mediaEntities.AssetAvailable,
	}
	jobs := []mediaEntities.CleanupJob{{
		ID:            "job",
		MediaAssetID:  retained.ID,
		Kind:          "retention",
		ClaimToken:    &claim,
		AttemptCount:  1,
		MaxAttempts:   8,
		NextAttemptAt: workerNow,
	}}

	repository.On("ExpirePendingAssets", mock.Anything, workerNow, workerBatchSize).
		Return([]mediaEntities.Asset{expired}, nil).Once()
	repository.On("CreateCleanupJob", mock.Anything, mock.MatchedBy(func(job *mediaEntities.CleanupJob) bool {
		return job.MediaAssetID == expired.ID && job.Kind == "failed_upload"
	})).Return(true, nil).Once()
	repository.On("ClaimCleanupJobs", mock.Anything, workerNow, cleanupLease, workerBatchSize).Return(jobs, nil).Once()
	repository.On("FindAsset", mock.Anything, retained.ID, false).Return(&retained, nil).Once()
	storage.On("DeleteObjectVersions", mock.Anything, "private", "staging/b").Return(nil).Once()
	storage.On("DeleteObjectVersions", mock.Anything, "private", "media/b.jpg").Return(nil).Once()
	repository.On("UpdateAsset", mock.Anything, mock.MatchedBy(func(asset *mediaEntities.Asset) bool {
		return asset.State == mediaEntities.AssetDeleted && asset.DeletedAt != nil
	})).Return(nil).Once()
	repository.On("CompleteCleanupJob", mock.Anything, "job", claim, workerNow).Return(nil).Once()
	repository.On("Metrics", mock.Anything, workerNow).Return(&mediaEntities.WorkerMetrics{Pending: 1}, nil).Once()

	worker := NewWorker(repository, storage)
	worker.now = func() time.Time { return workerNow }
	worker.jitter = func(time.Duration) time.Duration { return 0 }
	assert.NoError(t, worker.RunOnce(context.Background()))
}

func TestMediaWorker_ProviderFailureUsesPersistentBackoff(t *testing.T) {
	repository := mocks.NewMockMediaRepository(t)
	storage := mocks.NewMockMediaStorage(t)
	claim := "claim-token"
	asset := &mediaEntities.Asset{ID: "asset", Bucket: "private", StagingObjectKey: "staging/a"}
	job := mediaEntities.CleanupJob{
		ID:           "job",
		MediaAssetID: asset.ID,
		Kind:         "staging",
		ClaimToken:   &claim,
		AttemptCount: 3,
		MaxAttempts:  8,
	}

	repository.On("ExpirePendingAssets", mock.Anything, workerNow, workerBatchSize).
		Return([]mediaEntities.Asset{}, nil).Once()
	repository.On("ClaimCleanupJobs", mock.Anything, workerNow, cleanupLease, workerBatchSize).
		Return([]mediaEntities.CleanupJob{job}, nil).Once()
	repository.On("FindAsset", mock.Anything, asset.ID, false).Return(asset, nil).Once()
	storage.On("DeleteObjectVersions", mock.Anything, "private", "staging/a").
		Return(errors.New("secret provider error")).Once()
	repository.On("RetryCleanupJob", mock.Anything, "job", claim, workerNow.Add(8*time.Second), "provider", workerNow).
		Return(nil).Once()
	repository.On("Metrics", mock.Anything, workerNow).Return(&mediaEntities.WorkerMetrics{Retries: 1}, nil).Once()

	worker := NewWorker(repository, storage)
	worker.now = func() time.Time { return workerNow }
	worker.jitter = func(time.Duration) time.Duration { return 0 }
	assert.NoError(t, worker.RunOnce(context.Background()))
}

func TestMediaWorker_RecoversMissingAssetAndPropagatesCycleFailures(t *testing.T) {
	t.Run("missing asset completes historical job", func(t *testing.T) {
		repository := mocks.NewMockMediaRepository(t)
		storage := mocks.NewMockMediaStorage(t)
		claim := "claim"
		job := mediaEntities.CleanupJob{ID: "job", MediaAssetID: "missing", Kind: "retention", ClaimToken: &claim}
		repository.On("ExpirePendingAssets", mock.Anything, workerNow, workerBatchSize).
			Return([]mediaEntities.Asset{}, nil)
		repository.On("ClaimCleanupJobs", mock.Anything, workerNow, cleanupLease, workerBatchSize).
			Return([]mediaEntities.CleanupJob{job}, nil)
		repository.On("FindAsset", mock.Anything, "missing", false).Return(nil, appErrors.ErrNotFound)
		repository.On("CompleteCleanupJob", mock.Anything, "job", claim, workerNow).Return(nil)
		repository.On("Metrics", mock.Anything, workerNow).Return(&mediaEntities.WorkerMetrics{}, nil)
		worker := NewWorker(repository, storage)
		worker.now = func() time.Time { return workerNow }
		assert.NoError(t, worker.RunOnce(context.Background()))
	})

	t.Run("claim query failure", func(t *testing.T) {
		repository := mocks.NewMockMediaRepository(t)
		storage := mocks.NewMockMediaStorage(t)
		databaseErr := errors.New("database unavailable")
		repository.On("ExpirePendingAssets", mock.Anything, workerNow, workerBatchSize).
			Return([]mediaEntities.Asset{}, nil)
		repository.On("ClaimCleanupJobs", mock.Anything, workerNow, cleanupLease, workerBatchSize).
			Return(nil, databaseErr)
		worker := NewWorker(repository, storage)
		worker.now = func() time.Time { return workerNow }
		assert.ErrorIs(t, worker.RunOnce(context.Background()), databaseErr)
	})
}
