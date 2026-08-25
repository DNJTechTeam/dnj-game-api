package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
)

// TestMediaMoments_MediaRepositorySQLFailures exercises media_repository.go's error branches
// that require a genuinely broken connection (not reachable through the real-Postgres
// integration suite): each of Metrics' aggregate queries, its oldest-job lookup, and the
// idempotency-operation write paths.
func TestMediaMoments_MediaRepositorySQLFailures(t *testing.T) {
	t.Run("Metrics: an aggregate query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectQuery(`SELECT COUNT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.Metrics(context.Background(), time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CreateOperation: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		err := repo.CreateOperation(context.Background(), &entities.Operation{ID: "op-1"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CompleteOperation: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectExec(`UPDATE`).WillReturnError(errors.New("connection reset"))
		err := repo.CompleteOperation(context.Background(), "op-1", 200, nil, nil, nil, []byte(`{}`), time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("FindAsset: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.FindAsset(context.Background(), "asset-1", false)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("AcquireProcessingClaim: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectQuery(`INSERT INTO media_processing_claims`).WillReturnError(errors.New("connection reset"))
		_, _, err := repo.AcquireProcessingClaim(context.Background(), "asset-1", "token-1", "op-key", time.Now(), time.Now().Add(time.Minute))
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("UpdateProcessingClaim: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectExec(`UPDATE`).WillReturnError(errors.New("connection reset"))
		err := repo.UpdateProcessingClaim(context.Background(), &entities.ProcessingClaim{MediaAssetID: "asset-1", ClaimToken: "token-1"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CreateCleanupJob: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		_, err := repo.CreateCleanupJob(context.Background(), &entities.CleanupJob{ID: "job-1"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("ClaimCleanupJobs: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectQuery(`WITH candidates`).WillReturnError(errors.New("connection reset"))
		_, err := repo.ClaimCleanupJobs(context.Background(), time.Now(), time.Minute, 10)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CompleteCleanupJob: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectExec(`UPDATE`).WillReturnError(errors.New("connection reset"))
		err := repo.CompleteCleanupJob(context.Background(), "job-1", "token-1", time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("RetryCleanupJob: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectExec(`UPDATE`).WillReturnError(errors.New("connection reset"))
		err := repo.RetryCleanupJob(context.Background(), "job-1", "token-1", time.Now(), "provider", time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("ExpirePendingAssets: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](gormDB)}
		mock.ExpectQuery(`WITH candidates`).WillReturnError(errors.New("connection reset"))
		_, err := repo.ExpirePendingAssets(context.Background(), time.Now(), 10)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}

// TestMediaMoments_MomentRepositorySQLFailures exercises moment_repository.go's error branches
// that require a genuinely broken connection: the locked Participation/Activity lookups, the
// visibility/moderation listing queries, the like toggle, the challenge award ledger write, the
// award reversal's own lookups, moderation's asset/participation locks, and the moderation
// decision write — none of which the real-Postgres integration suite can force.
func TestMediaMoments_MomentRepositorySQLFailures(t *testing.T) {
	t.Run("FindParticipationForUpdate: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.FindParticipationForUpdate(context.Background(), "participation-1")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("FindActivityForUpdate: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, _, _, _, _, _, _, err := repo.FindActivityForUpdate(context.Background(), "activity-1")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("ListMoments: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.ListMoments(context.Background(), "mine", 1, nil, nil, time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("ToggleLike: a generic lookup failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, _, err := repo.ToggleLike(context.Background(), "moment-1", 1, time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("ListModeration: a generic query failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.ListModeration(context.Background(), "general", 0, time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("AwardMoment: a generic ledger write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectExec(`INSERT INTO point_entries`).WillReturnError(errors.New("connection reset"))
		err := repo.AwardMoment(context.Background(), "moment-1", 1, "activity-1", 10, time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("ReverseMomentAward: a generic moment lookup failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.ReverseMomentAward(context.Background(), "moment-1", 1, time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("ApplyModeration: a generic asset reference lookup failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, _, _, err := repo.ApplyModeration(context.Background(), "moment-1", "deny_points", 1, "key", time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CreateModerationDecision: a generic write failure is redacted", func(t *testing.T) {
		gormDB, mock := newMockDB(t)
		repo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](gormDB)}
		mock.ExpectBegin().WillReturnError(errors.New("connection reset"))
		_, err := repo.CreateModerationDecision(context.Background(), &momentEntities.ModerationDecision{ID: "d-1"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
