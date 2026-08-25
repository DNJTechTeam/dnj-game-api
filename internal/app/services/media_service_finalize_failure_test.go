package services

import (
	"errors"
	"net/http"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestMediaMoments_FinalizeUploadRepositoryFailures exercises finalizeUpload's internal
// failure branches directly — the asset-lock lookup, ownership mismatch, re-validated actor,
// the final asset persistence, the idempotency completion write, and the terminating claim
// update — none of which are reliably forceable through a real Postgres round-trip.
func TestMediaMoments_FinalizeUploadRepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)
	asset := &mediaEntities.Asset{ID: "asset-1", OwnerUserID: 42, Bytes: 10, ChecksumSHA256: "abc"}
	op := &mediaEntities.Operation{ID: "op-1"}
	claim := &mediaEntities.ProcessingClaim{MediaAssetID: "asset-1", ClaimToken: "token-1"}

	t.Run("locked asset lookup generic failure is redacted", func(t *testing.T) {
		service, repo, _, _ := newMediaServiceWithMocks(t)
		repo.On("FindAsset", mock.Anything, "asset-1", true).Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.finalizeUpload(ctx, 42, asset, op, claim)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("foreign asset ownership is a uniform not-found", func(t *testing.T) {
		service, repo, _, _ := newMediaServiceWithMocks(t)
		repo.On("FindAsset", mock.Anything, "asset-1", true).
			Return(&mediaEntities.Asset{ID: "asset-1", OwnerUserID: 999}, nil).Once()
		_, _, err := service.finalizeUpload(ctx, 42, asset, op, claim)
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Status)
	})

	t.Run("re-validated actor failure is propagated", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		repo.On("FindAsset", mock.Anything, "asset-1", true).
			Return(&mediaEntities.Asset{ID: "asset-1", OwnerUserID: 42}, nil).Once()
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.finalizeUpload(ctx, 42, asset, op, claim)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("final asset persistence generic failure is redacted", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "asset-1", true).
			Return(&mediaEntities.Asset{ID: "asset-1", OwnerUserID: 42}, nil).Once()
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).
			Return(errors.New("connection reset")).Once()
		_, _, err := service.finalizeUpload(ctx, 42, asset, op, claim)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("idempotency completion generic failure is redacted", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "asset-1", true).
			Return(&mediaEntities.Asset{ID: "asset-1", OwnerUserID: 42}, nil).Once()
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).Return(nil).Once()
		repo.On("CreateCleanupJob", mock.Anything, mock.AnythingOfType("*entities.CleanupJob")).Return(true, nil).Once()
		repo.On("CompleteOperation", mock.Anything, "op-1", http.StatusOK, mock.Anything, (*bool)(nil), (*int)(nil), mock.Anything, mock.Anything).
			Return(errors.New("connection reset")).Once()
		_, _, err := service.finalizeUpload(ctx, 42, asset, op, claim)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("lost lease on the terminating claim update is propagated", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "asset-1", true).
			Return(&mediaEntities.Asset{ID: "asset-1", OwnerUserID: 42}, nil).Once()
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).Return(nil).Once()
		repo.On("CreateCleanupJob", mock.Anything, mock.AnythingOfType("*entities.CleanupJob")).Return(true, nil).Once()
		repo.On("CompleteOperation", mock.Anything, "op-1", http.StatusOK, mock.Anything, (*bool)(nil), (*int)(nil), mock.Anything, mock.Anything).
			Return(nil).Once()
		repo.On("UpdateProcessingClaim", mock.Anything, mock.AnythingOfType("*entities.ProcessingClaim")).
			Return(appErrors.ErrConflict).Once()
		_, _, err := service.finalizeUpload(ctx, 42, asset, op, claim)
		assert.ErrorIs(t, err, appErrors.ErrConflict)
	})
}

// TestMediaMoments_CompleteTerminalAndFailInvalidRepositoryFailures exercises the
// terminal-failure recording path (completeTerminal, invoked by failInvalid) that marks an
// asset "failed" and completes its idempotency operation with a redacted snapshot — including
// its own internal repository-failure branches.
func TestMediaMoments_CompleteTerminalAndFailInvalidRepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)

	t.Run("asset persistence failure is redacted", func(t *testing.T) {
		service, repo, _, _ := newMediaServiceWithMocks(t)
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).
			Return(errors.New("connection reset")).Once()
		asset := &mediaEntities.Asset{ID: "asset-1"}
		err := service.completeTerminal(ctx, asset, &mediaEntities.Operation{ID: "op-1"}, http.StatusUnprocessableEntity, "IMAGE_INVALID", "bad", time.Now())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("nil operation short-circuits without completing an operation", func(t *testing.T) {
		service, repo, _, _ := newMediaServiceWithMocks(t)
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).Return(nil).Once()
		repo.On("CreateCleanupJob", mock.Anything, mock.AnythingOfType("*entities.CleanupJob")).Return(true, nil).Once()
		asset := &mediaEntities.Asset{ID: "asset-1"}
		err := service.completeTerminal(ctx, asset, nil, http.StatusUnprocessableEntity, "IMAGE_INVALID", "bad", time.Now())
		assert.NoError(t, err)
	})

	t.Run("failInvalid propagates a transactional failure as an internal error", func(t *testing.T) {
		service, repo, _, _ := newMediaServiceWithMocks(t)
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).
			Return(errors.New("connection reset")).Once()
		repo.On("UpdateProcessingClaim", mock.Anything, mock.AnythingOfType("*entities.ProcessingClaim")).
			Return(nil).Once()
		asset := &mediaEntities.Asset{ID: "asset-1"}
		claim := &mediaEntities.ProcessingClaim{MediaAssetID: "asset-1", ClaimToken: "token-1"}
		_, _, err := service.failInvalid(ctx, asset, &mediaEntities.Operation{ID: "op-1"}, claim, http.StatusUnprocessableEntity, "IMAGE_INVALID", "bad")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
