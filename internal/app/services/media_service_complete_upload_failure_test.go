package services

import (
	"errors"
	"net/http"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestMediaMoments_CompleteUploadRepositoryFailures exercises CompleteUpload's internal
// repository-failure branches that a real Postgres round-trip cannot reliably force: a generic
// asset-lock failure, a re-validated actor failure, a generic idempotency-operation lookup
// failure, a generic idempotency-operation persistence failure, a failure recording the
// UPLOAD_EXPIRED terminal state, and a generic claim-acquisition failure. It also covers the
// legitimate "already being confirmed under a different key" conflict.
func TestMediaMoments_CompleteUploadRepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)
	key := "22222222-2222-4222-8222-222222222222"

	t.Run("asset lock generic failure is redacted", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).
			Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.CompleteUpload(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("re-validated actor failure is propagated", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		users.On("FindByID", mock.Anything, uint64(42)).
			Return(&userEntities.User{ID: 42, Role: userEntities.RoleDefault, OnboardingComplete: true}, nil).Once()
		repo.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).
			Return(&mediaEntities.Asset{ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42}, nil).Once()
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.CompleteUpload(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("generic idempotency operation lookup failure is redacted", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).
			Return(&mediaEntities.Asset{ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42}, nil).Once()
		repo.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.CompleteUpload(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("idempotency operation persistence generic failure is redacted", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).
			Return(&mediaEntities.Asset{
				ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42,
				State: mediaEntities.AssetPendingUpload, UploadExpiresAt: time.Now().Add(time.Hour),
			}, nil).Once()
		repo.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		repo.On("CreateOperation", mock.Anything, mock.AnythingOfType("*entities.Operation")).
			Return(errors.New("connection reset")).Once()
		_, _, err := service.CompleteUpload(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("expired intent whose terminal persistence fails surfaces the underlying failure", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).
			Return(&mediaEntities.Asset{
				ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42,
				State: mediaEntities.AssetPendingUpload, UploadExpiresAt: time.Now().Add(-time.Hour),
			}, nil).Once()
		repo.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		repo.On("CreateOperation", mock.Anything, mock.AnythingOfType("*entities.Operation")).Return(nil).Once()
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).
			Return(errors.New("connection reset")).Once()
		_, _, err := service.CompleteUpload(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("claim acquisition generic failure is redacted", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).
			Return(&mediaEntities.Asset{
				ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42,
				State: mediaEntities.AssetPendingUpload, UploadExpiresAt: time.Now().Add(time.Hour),
			}, nil).Once()
		repo.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		repo.On("CreateOperation", mock.Anything, mock.AnythingOfType("*entities.Operation")).Return(nil).Once()
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).Return(nil).Once()
		repo.On("AcquireProcessingClaim", mock.Anything, "11111111-1111-4111-8111-111111111111", mock.Anything, key, mock.Anything, mock.Anything).
			Return(nil, false, errors.New("connection reset")).Once()
		_, _, err := service.CompleteUpload(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("confirmation already in progress under a different key is a conflict", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).
			Return(&mediaEntities.Asset{
				ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42,
				State: mediaEntities.AssetPendingUpload, UploadExpiresAt: time.Now().Add(time.Hour),
			}, nil).Once()
		repo.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		repo.On("CreateOperation", mock.Anything, mock.AnythingOfType("*entities.Operation")).Return(nil).Once()
		repo.On("UpdateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).Return(nil).Once()
		repo.On("AcquireProcessingClaim", mock.Anything, "11111111-1111-4111-8111-111111111111", mock.Anything, key, mock.Anything, mock.Anything).
			Return(&mediaEntities.ProcessingClaim{MediaAssetID: "11111111-1111-4111-8111-111111111111", OperationKey: "other-key"}, false, nil).Once()
		_, _, err := service.CompleteUpload(ctx, "11111111-1111-4111-8111-111111111111", key)
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusConflict, apiErr.Status)
		assert.Equal(t, "UPLOAD_STATE_CONFLICT", apiErr.Code)
	})
}
