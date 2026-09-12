package services

import (
	"errors"
	"net/http"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestMediaMoments_CreateMomentRepositoryFailures exercises Create's internal repository
// failure branches that a real Postgres round-trip cannot reliably force: a generic asset
// lookup failure and a replayed idempotency record whose referenced Moment lookup fails.
func TestMediaMoments_CreateMomentRepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)
	key := "22222222-2222-4222-8222-222222222222"
	request := &messages.CreateMomentRequestDTO{MediaAssetID: "11111111-1111-4111-8111-111111111111", PublishConsent: true}

	t.Run("asset lookup generic failure is redacted", func(t *testing.T) {
		service, _, media, users, _ := newMomentServiceWithMocks(t)
		mockDefaultActor(users, 42)
		media.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", false).
			Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.Create(ctx, key, request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("replayed record whose referenced moment lookup fails is redacted", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockDefaultActor(users, 42)
		asset := &mediaEntities.Asset{ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42, State: mediaEntities.AssetAvailable}
		media.On("FindAsset", mock.Anything, "11111111-1111-4111-8111-111111111111", false).Return(asset, nil).Once()
		fingerprint := intentHash("moment.create", createMomentIntent{MediaAssetID: asset.ID, PublishConsent: true})
		resultRef := "moment-1"
		media.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&mediaEntities.Operation{Operation: "moment.create", IntentHash: fingerprint, ResultRef: &resultRef}, nil).Once()
		moments.On("FindMoment", mock.Anything, "moment-1", uint64(42), false).
			Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.Create(ctx, key, request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}

// TestMediaMoments_CreateMomentDuplicateReplaysRacingRequest pins the lock-free
// publish contract: when the Moment insert hits a unique index because a racing
// request with the same Idempotency-Key already committed, Create replays that
// request's result instead of reporting a conflict against itself.
func TestMediaMoments_CreateMomentDuplicateReplaysRacingRequest(t *testing.T) {
	ctx := ctxWithUser(42)
	key := "33333333-3333-4333-8333-333333333333"
	request := &messages.CreateMomentRequestDTO{MediaAssetID: "11111111-1111-4111-8111-111111111111", PublishConsent: true}
	service, moments, media, users, _ := newMomentServiceWithMocks(t)
	mockDefaultActor(users, 42)
	asset := &mediaEntities.Asset{
		ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: 42, State: mediaEntities.AssetAvailable,
		RetentionDueAt: time.Now().Add(time.Hour),
	}
	media.On("FindAsset", mock.Anything, asset.ID, false).Return(asset, nil)
	fingerprint := intentHash("moment.create", createMomentIntent{MediaAssetID: asset.ID, PublishConsent: true})
	// Before the insert nothing is recorded for the key; once the unique
	// violation surfaces, the racing request has committed and is visible.
	media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
	media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
	resultRef := "moment-1"
	media.On("FindOperation", mock.Anything, uint64(42), key).Return(&mediaEntities.Operation{
		Operation: "moment.create", IntentHash: fingerprint, ResultRef: &resultRef, HTTPStatus: http.StatusCreated,
	}, nil)
	moments.On("CreateMoment", mock.Anything, mock.Anything).Return(appErrors.ErrConflict).Once()
	moments.On("FindMoment", mock.Anything, "moment-1", uint64(42), false).
		Return(&momentEntities.Moment{ID: "moment-1", UserID: 42, MediaAssetID: asset.ID, Origin: momentEntities.OriginFree}, nil).Once()

	response, status, err := service.Create(ctx, key, request)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "moment-1", response.ID)
}
