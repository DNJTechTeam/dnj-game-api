package services

import (
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMediaMoments_CreateMomentRepositoryFailures exercises Create's internal repository
// failure branches that a real Postgres round-trip cannot reliably force: a generic asset-lock
// failure, a replayed idempotency record whose referenced Moment lookup fails, and a
// re-validated actor failure.
func TestMediaMoments_CreateMomentRepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)
	key := "22222222-2222-4222-8222-222222222222"
	request := &messages.CreateMomentRequestDTO{MediaAssetID: "11111111-1111-4111-8111-111111111111", PublishConsent: true}

	t.Run("asset lock generic failure is redacted", func(t *testing.T) {
		service, _, media, users, _ := newMomentServiceWithMocks(t)
		mockDefaultActor(users, 42)
		media.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).
			Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.Create(ctx, key, request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("replayed record whose referenced moment lookup fails is redacted", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockDefaultActor(users, 42)
		asset := &mediaEntities.Asset{ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42, State: mediaEntities.AssetAvailable}
		media.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).Return(asset, nil).Once()
		fingerprint := intentHash("moment.create", createMomentIntent{MediaAssetID: asset.ID, PublishConsent: true})
		resultRef := "moment-1"
		media.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&mediaEntities.Operation{Operation: "moment.create", IntentHash: fingerprint, ResultRef: &resultRef}, nil).Once()
		moments.On("FindMoment", mock.Anything, "moment-1", uint64(42), false).
			Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.Create(ctx, key, request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("re-validated actor failure is propagated for a free moment", func(t *testing.T) {
		service, _, media, users, _ := newMomentServiceWithMocks(t)
		users.On("FindByID", mock.Anything, uint64(42)).
			Return(&userEntities.User{ID: 42, Role: userEntities.RoleDefault, OnboardingComplete: true}, nil).Once()
		asset := &mediaEntities.Asset{
			ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42, State: mediaEntities.AssetAvailable,
			RetentionDueAt: time.Now().Add(time.Hour),
		}
		media.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", true).Return(asset, nil).Once()
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.Create(ctx, key, request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
