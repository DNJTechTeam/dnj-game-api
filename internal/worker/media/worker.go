package media

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
	"github.com/google/uuid"
)

const (
	workerBatchSize = 50
	cleanupLease    = 2 * time.Minute
)

type Worker struct {
	repository mediaInterfaces.Repository
	storage    mediaInterfaces.Storage
	now        func() time.Time
	jitter     func(time.Duration) time.Duration
}

func NewWorker(repository mediaInterfaces.Repository, storage mediaInterfaces.Storage) *Worker {
	return &Worker{
		repository: repository,
		storage:    storage,
		now:        time.Now,
		jitter: func(base time.Duration) time.Duration {
			return time.Duration(rand.Int63n(int64(base/4) + 1))
		},
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	now := w.now().UTC()
	expiredAssets, err := w.repository.ExpirePendingAssets(ctx, now, workerBatchSize)
	if err != nil {
		return err
	}
	for index := range expiredAssets {
		asset := &expiredAssets[index]
		_, err = w.repository.CreateCleanupJob(ctx, &mediaEntities.CleanupJob{
			ID:            uuid.NewString(),
			MediaAssetID:  asset.ID,
			Kind:          "failed_upload",
			State:         "pending",
			DueAt:         now,
			MaxAttempts:   8,
			NextAttemptAt: now,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			return err
		}
	}

	jobs, err := w.repository.ClaimCleanupJobs(ctx, now, cleanupLease, workerBatchSize)
	if err != nil {
		return err
	}
	for index := range jobs {
		w.processJob(ctx, &jobs[index])
	}

	metrics, err := w.repository.Metrics(ctx, now)
	if err != nil {
		return err
	}
	slog.InfoContext(
		ctx,
		"media cleanup cycle completed",
		"pending", metrics.Pending,
		"processing", metrics.Processing,
		"expired", metrics.Expired,
		"failed", metrics.Failed,
		"retries", metrics.Retries,
		"oldest_job_age_seconds", metrics.OldestJobAgeSeconds,
	)
	return nil
}

func (w *Worker) processJob(ctx context.Context, job *mediaEntities.CleanupJob) {
	if job.ClaimToken == nil {
		return
	}
	now := w.now().UTC()
	asset, err := w.repository.FindAsset(ctx, job.MediaAssetID, false)
	if errors.Is(err, appErrors.ErrNotFound) {
		if completeErr := w.repository.CompleteCleanupJob(ctx, job.ID, *job.ClaimToken, now); completeErr != nil {
			w.retry(ctx, job, "database")
		}
		return
	}
	if err != nil {
		w.retry(ctx, job, "database")
		return
	}

	if err := w.deleteJobObjects(ctx, job.Kind, asset); err != nil {
		w.retry(ctx, job, "provider")
		return
	}
	if job.Kind != "staging" && asset.State != mediaEntities.AssetDeleted {
		asset.State = mediaEntities.AssetDeleted
		asset.DeletedAt = &now
		asset.UpdatedAt = now
		if err := w.repository.UpdateAsset(ctx, asset); err != nil {
			w.retry(ctx, job, "database")
			return
		}
	}
	if err := w.repository.CompleteCleanupJob(ctx, job.ID, *job.ClaimToken, now); err != nil {
		w.retry(ctx, job, "database")
	}
}

func (w *Worker) deleteJobObjects(
	ctx context.Context,
	kind string,
	asset *mediaEntities.Asset,
) error {
	if err := w.storage.DeleteObjectVersions(ctx, asset.Bucket, asset.StagingObjectKey); err != nil {
		return err
	}
	if kind == "staging" {
		return nil
	}
	return w.storage.DeleteObjectVersions(ctx, asset.Bucket, asset.FinalObjectKey)
}

func (w *Worker) retry(ctx context.Context, job *mediaEntities.CleanupJob, code string) {
	base := time.Second << min(job.AttemptCount, 12)
	if base > time.Hour {
		base = time.Hour
	}
	now := w.now().UTC()
	nextAttempt := now.Add(base + w.jitter(base))
	_ = w.repository.RetryCleanupJob(
		ctx,
		job.ID,
		*job.ClaimToken,
		nextAttempt,
		code,
		now,
	)
}
