package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MediaRepository struct {
	*BaseRepository[models.MediaAsset]
}

func NewMediaRepository(db *gorm.DB) mediaInterfaces.Repository {
	return &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](db)}
}

func (r *MediaRepository) CreateAsset(ctx context.Context, asset *entities.Asset) error {
	return r.BaseRepository.Create(ctx, mappers.MapMediaAssetEntityToModel(asset))
}
func (r *MediaRepository) FindAsset(ctx context.Context, id string, lock bool) (*entities.Asset, error) {
	var row models.MediaAsset
	query := r.getDB(ctx).Where("id = ?", id)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapMediaAssetToEntity(&row), nil
}
func (r *MediaRepository) UpdateAsset(ctx context.Context, asset *entities.Asset) error {
	return r.BaseRepository.Update(ctx, mappers.MapMediaAssetEntityToModel(asset))
}

func (r *MediaRepository) FindOperation(ctx context.Context, actor uint64, key string) (*entities.Operation, error) {
	var row models.IdempotencyOperation
	if err := r.getDB(ctx).Where("actor_user_id = ? AND idempotency_key = ?", actor, key).First(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapOperationToEntity(&row), nil
}
func (r *MediaRepository) CreateOperation(ctx context.Context, operation *entities.Operation) error {
	snapshot := json.RawMessage(operation.ResponseSnapshot)
	if len(snapshot) == 0 {
		snapshot = json.RawMessage(`{}`)
	}
	row := &models.IdempotencyOperation{
		ID:               operation.ID,
		ActorUserID:      operation.ActorUserID,
		IdempotencyKey:   operation.IdempotencyKey,
		Operation:        operation.Operation,
		ResourceRef:      operation.ResourceRef,
		IntentHash:       operation.IntentHash,
		State:            operation.State,
		ResultRef:        operation.ResultRef,
		ResultBoolean:    operation.ResultBoolean,
		ResultCount:      operation.ResultCount,
		ResponseSnapshot: snapshot,
		HTTPStatus:       operation.HTTPStatus,
		CreatedAt:        operation.CreatedAt,
		CompletedAt:      operation.CompletedAt,
	}
	return handleRepositoryError(r.getDB(ctx).Create(row).Error)
}

func (r *MediaRepository) CompleteOperation(
	ctx context.Context,
	id string,
	status int,
	ref *string,
	value *bool,
	count *int,
	snapshot []byte,
	now time.Time,
) error {
	if len(snapshot) == 0 {
		snapshot = []byte(`{}`)
	}
	result := r.getDB(ctx).
		Model(&models.IdempotencyOperation{}).
		Where("id = ? AND state = 'processing'", id).
		Updates(map[string]any{"state": "completed", "result_ref": ref, "result_boolean": value, "result_count": count, "response_snapshot": json.RawMessage(snapshot), "http_status": status, "completed_at": now})
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	return nil
}
func (r *MediaRepository) FindLegacyOperation(ctx context.Context, actor uint64, key string) (bool, error) {
	var exists bool
	err := r.getDB(ctx).Raw(`SELECT EXISTS (
		SELECT 1 FROM participant_operations WHERE actor_user_id = ? AND idempotency_key = ?
		UNION ALL SELECT 1 FROM manager_operations WHERE actor_user_id = ? AND idempotency_key = ?
		UNION ALL SELECT 1 FROM admin_operations WHERE actor_user_id = ? AND idempotency_key = ?
	)`, actor, key, actor, key, actor, key).Scan(&exists).Error
	return exists, handleRepositoryError(err)
}

func mapClaim(row *models.MediaProcessingClaim) *entities.ProcessingClaim {
	if row == nil {
		return nil
	}
	return &entities.ProcessingClaim{
		MediaAssetID:      row.MediaAssetID,
		ClaimToken:        row.ClaimToken,
		OperationKey:      row.OperationKey,
		Stage:             row.Stage,
		StagingVersionID:  row.StagingVersionID,
		FinalVersionID:    row.FinalVersionID,
		LeaseExpiresAt:    row.LeaseExpiresAt,
		AttemptCount:      row.AttemptCount,
		LastErrorCategory: row.LastErrorCategory,
		CompletedAt:       row.CompletedAt,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func (r *MediaRepository) AcquireProcessingClaim(
	ctx context.Context,
	assetID, token, operationKey string,
	now, lease time.Time,
) (*entities.ProcessingClaim, bool, error) {
	var row models.MediaProcessingClaim
	err := r.getDB(ctx).
		Raw(`INSERT INTO media_processing_claims (media_asset_id,claim_token,operation_key,stage,lease_expires_at,attempt_count,created_at,updated_at)
		VALUES (?, ?, ?, 'claimed', ?, 1, ?, ?)
		ON CONFLICT (media_asset_id) DO UPDATE SET claim_token=EXCLUDED.claim_token, operation_key=EXCLUDED.operation_key,
		 lease_expires_at=EXCLUDED.lease_expires_at, attempt_count=media_processing_claims.attempt_count+1, updated_at=EXCLUDED.updated_at
		WHERE media_processing_claims.completed_at IS NULL AND media_processing_claims.lease_expires_at <= ?
		RETURNING *`, assetID, token, operationKey, lease, now, now, now).
		Scan(&row).
		Error
	if err != nil {
		return nil, false, handleRepositoryError(err)
	}
	if row.ClaimToken == "" {
		if err := r.getDB(ctx).Where("media_asset_id = ?", assetID).First(&row).Error; err != nil {
			return nil, false, handleRepositoryError(err)
		}
		return mapClaim(&row), false, nil
	}
	return mapClaim(&row), true, nil
}
func (r *MediaRepository) UpdateProcessingClaim(ctx context.Context, claim *entities.ProcessingClaim) error {
	result := r.getDB(ctx).
		Model(&models.MediaProcessingClaim{}).
		Where("media_asset_id = ? AND claim_token = ?", claim.MediaAssetID, claim.ClaimToken).
		Updates(map[string]any{"stage": claim.Stage, "staging_version_id": claim.StagingVersionID, "final_version_id": claim.FinalVersionID, "lease_expires_at": claim.LeaseExpiresAt, "last_error_category": claim.LastErrorCategory, "completed_at": claim.CompletedAt, "updated_at": claim.UpdatedAt})
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	return nil
}

func mapCleanup(row *models.MediaCleanupJob) entities.CleanupJob {
	return entities.CleanupJob{
		ID:             row.ID,
		MediaAssetID:   row.MediaAssetID,
		Kind:           row.Kind,
		State:          row.State,
		DueAt:          row.DueAt,
		AttemptCount:   row.AttemptCount,
		MaxAttempts:    row.MaxAttempts,
		NextAttemptAt:  row.NextAttemptAt,
		ClaimToken:     row.ClaimToken,
		LeaseExpiresAt: row.LeaseExpiresAt,
		LastErrorCode:  row.LastErrorCode,
		CompletedAt:    row.CompletedAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
func (r *MediaRepository) CreateCleanupJob(ctx context.Context, job *entities.CleanupJob) (bool, error) {
	row := &models.MediaCleanupJob{
		ID:            job.ID,
		MediaAssetID:  job.MediaAssetID,
		Kind:          job.Kind,
		State:         job.State,
		DueAt:         job.DueAt,
		AttemptCount:  job.AttemptCount,
		MaxAttempts:   job.MaxAttempts,
		NextAttemptAt: job.NextAttemptAt,
		CreatedAt:     job.CreatedAt,
		UpdatedAt:     job.UpdatedAt,
	}
	result := r.getDB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *MediaRepository) ClaimCleanupJobs(
	ctx context.Context,
	now time.Time,
	lease time.Duration,
	limit int,
) ([]entities.CleanupJob, error) {
	var rows []models.MediaCleanupJob
	err := r.getDB(ctx).
		Raw(`WITH candidates AS (SELECT id FROM media_cleanup_jobs WHERE state IN ('pending','retry','processing') AND due_at <= ? AND next_attempt_at <= ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?) ORDER BY due_at,id FOR UPDATE SKIP LOCKED LIMIT ?)
		UPDATE media_cleanup_jobs SET state='processing', claim_token=gen_random_uuid(), lease_expires_at=?, attempt_count=attempt_count+1, updated_at=? WHERE id IN (SELECT id FROM candidates) RETURNING *`, now, now, now, limit, now.Add(lease), now).
		Scan(&rows).
		Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	result := make([]entities.CleanupJob, len(rows))
	for i := range rows {
		result[i] = mapCleanup(&rows[i])
	}
	return result, nil
}
func (r *MediaRepository) CompleteCleanupJob(ctx context.Context, id, token string, now time.Time) error {
	result := r.getDB(ctx).
		Model(&models.MediaCleanupJob{}).
		Where("id=? AND claim_token=? AND state='processing'", id, token).
		Updates(map[string]any{"state": "completed", "completed_at": now, "lease_expires_at": nil, "updated_at": now})
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	return nil
}

func (r *MediaRepository) RetryCleanupJob(
	ctx context.Context,
	id, token string,
	next time.Time,
	code string,
	now time.Time,
) error {
	result := r.getDB(ctx).
		Model(&models.MediaCleanupJob{}).
		Where("id=? AND claim_token=? AND state='processing'", id, token).
		Updates(map[string]any{"state": gorm.Expr("CASE WHEN attempt_count >= max_attempts THEN 'failed' ELSE 'retry' END"), "next_attempt_at": next, "last_error_code": code, "lease_expires_at": nil, "updated_at": now})
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	return nil
}
func (r *MediaRepository) ExpirePendingAssets(ctx context.Context, now time.Time, limit int) ([]entities.Asset, error) {
	var rows []models.MediaAsset
	err := r.getDB(ctx).
		Raw(`WITH candidates AS (
			SELECT media_assets.id FROM media_assets
			LEFT JOIN media_processing_claims ON media_processing_claims.media_asset_id = media_assets.id
			WHERE media_assets.state IN ('pending_upload','processing')
			  AND media_assets.upload_expires_at < ?
			  AND (media_processing_claims.media_asset_id IS NULL OR media_processing_claims.completed_at IS NOT NULL OR media_processing_claims.lease_expires_at <= ?)
			ORDER BY media_assets.upload_expires_at,media_assets.id
			FOR UPDATE OF media_assets SKIP LOCKED LIMIT ?
		) UPDATE media_assets SET state='failed',failed_at=?,updated_at=?
		WHERE id IN (SELECT id FROM candidates) RETURNING *`, now, now, limit, now, now).
		Scan(&rows).
		Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	out := make([]entities.Asset, len(rows))
	for i := range rows {
		out[i] = *mappers.MapMediaAssetToEntity(&rows[i])
	}
	return out, nil
}
func (r *MediaRepository) Metrics(ctx context.Context, now time.Time) (*entities.WorkerMetrics, error) {
	metrics := &entities.WorkerMetrics{}
	queries := []struct {
		target *int64
		sql    string
		args   []any
	}{
		{&metrics.Pending, `SELECT COUNT(*) FROM media_cleanup_jobs WHERE state IN ('pending','retry')`, nil},
		{&metrics.Processing, `SELECT COUNT(*) FROM media_cleanup_jobs WHERE state='processing'`, nil},
		{&metrics.Expired, `SELECT COUNT(*) FROM media_assets WHERE state <> 'deleted' AND retention_due_at <= ?`, []any{now}},
		{&metrics.Failed, `SELECT COUNT(*) FROM media_cleanup_jobs WHERE state='failed'`, nil},
		{&metrics.Retries, `SELECT COALESCE(SUM(GREATEST(attempt_count-1,0)),0) FROM media_cleanup_jobs`, nil},
	}
	for _, q := range queries {
		if err := r.getDB(ctx).Raw(q.sql, q.args...).Scan(q.target).Error; err != nil {
			return nil, handleRepositoryError(err)
		}
	}
	var oldest sql.NullTime
	if err := r.getDB(ctx).Raw(`SELECT MIN(due_at) FROM media_cleanup_jobs WHERE state IN ('pending','retry','processing')`).Scan(&oldest).Error; err != nil &&
		!errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, handleRepositoryError(err)
	}
	if oldest.Valid {
		metrics.OldestJobAgeSeconds = now.Sub(oldest.Time).Seconds()
		if metrics.OldestJobAgeSeconds < 0 {
			metrics.OldestJobAgeSeconds = 0
		}
	}
	return metrics, nil
}
