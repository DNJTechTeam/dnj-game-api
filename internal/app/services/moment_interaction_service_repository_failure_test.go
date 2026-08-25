package services

import (
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newMomentServiceWithMocks(t *testing.T) (
	*MomentService,
	*mocks.MockMomentRepository,
	*mocks.MockMediaRepository,
	*mocks.MockUserRepositoryInterface,
	*mocks.MockOperationAuditRepositoryInterface,
) {
	t.Helper()
	TestSuite.DefaultSetup(t)
	moments := mocks.NewMockMomentRepository(t)
	media := mocks.NewMockMediaRepository(t)
	storage := mocks.NewMockMediaStorage(t)
	users := mocks.NewMockUserRepositoryInterface(t)
	audits := mocks.NewMockOperationAuditRepositoryInterface(t)
	service := NewMomentService(TestSuite.BaseService, moments, media, storage, users, audits).(*MomentService)
	return service, moments, media, users, audits
}

func mockAdminActor(users *mocks.MockUserRepositoryInterface, id uint64) *userEntities.User {
	actor := &userEntities.User{ID: id, Role: userEntities.RoleAdmin, OnboardingComplete: true}
	users.On("FindByID", mock.Anything, id).Return(actor, nil).Maybe()
	return actor
}

var visibleLikeableMoment = &momentEntities.Moment{
	ID: "moment-1", MediaAssetID: "asset-1", PublicationStatus: momentEntities.PublicationPublic,
	ModerationStatus: momentEntities.ModerationApproved, AssetAvailable: true, AuthorEligible: true,
	AssetRetentionDueAt: time.Now().Add(time.Hour),
}

// TestMediaMoments_ToggleLikeRepositoryFailures exercises ToggleLike's internal repository
// failure branches: a stale replayed idempotency record missing its cached result, a generic
// idempotency-lookup failure, a re-validated actor failure, a like-toggle repository failure,
// and a generic idempotency-persistence failure — none of which a real Postgres round-trip can
// reliably force.
func TestMediaMoments_ToggleLikeRepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)
	key := "22222222-2222-4222-8222-222222222222"

	t.Run("a replayed record without a cached result is a safe internal error", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockDefaultActor(users, 42)
		moments.On("FindMoment", mock.Anything, "11111111-1111-4111-8111-111111111111", uint64(42), true).
			Return(visibleLikeableMoment, nil).Once()
		media.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&mediaEntities.Operation{Operation: "moment.like.toggle", IntentHash: intentHash("moment.like.toggle", struct {
				MomentID string `json:"momentId"`
			}{MomentID: "11111111-1111-4111-8111-111111111111"})}, nil).Once()
		_, err := service.ToggleLike(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("generic idempotency lookup failure is redacted", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockDefaultActor(users, 42)
		moments.On("FindMoment", mock.Anything, "11111111-1111-4111-8111-111111111111", uint64(42), true).
			Return(visibleLikeableMoment, nil).Once()
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, errors.New("connection reset")).Once()
		_, err := service.ToggleLike(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("re-validated actor failure is propagated", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		users.On("FindByID", mock.Anything, uint64(42)).
			Return(&userEntities.User{ID: 42, Role: userEntities.RoleDefault, OnboardingComplete: true}, nil).Once()
		moments.On("FindMoment", mock.Anything, "11111111-1111-4111-8111-111111111111", uint64(42), true).
			Return(visibleLikeableMoment, nil).Once()
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(nil, errors.New("connection reset")).Once()
		_, err := service.ToggleLike(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("like toggle repository failure is redacted", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockDefaultActor(users, 42)
		moments.On("FindMoment", mock.Anything, "11111111-1111-4111-8111-111111111111", uint64(42), true).
			Return(visibleLikeableMoment, nil).Once()
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		moments.On("ToggleLike", mock.Anything, "moment-1", uint64(42), mock.Anything).
			Return(false, 0, errors.New("connection reset")).Once()
		_, err := service.ToggleLike(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("idempotency persistence generic failure is redacted", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockDefaultActor(users, 42)
		moments.On("FindMoment", mock.Anything, "11111111-1111-4111-8111-111111111111", uint64(42), true).
			Return(visibleLikeableMoment, nil).Once()
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		moments.On("ToggleLike", mock.Anything, "moment-1", uint64(42), mock.Anything).Return(true, 1, nil).Once()
		media.On("CreateOperation", mock.Anything, mock.AnythingOfType("*entities.Operation")).
			Return(errors.New("connection reset")).Once()
		_, err := service.ToggleLike(ctx, "11111111-1111-4111-8111-111111111111", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}

// TestMediaMoments_ModerateRepositoryFailures exercises Moderate's replay path and
// recordModerationEffect's internal repository-failure branches: a failed replay lookup, a
// moderation-decision persistence failure, a durable-no-op replay of an already-applied
// decision, an audit-write failure, and a cleanup-enqueue failure for delete_photo.
func TestMediaMoments_ModerateRepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)

	t.Run("replayModeration surfaces a failed moment lookup as an internal error", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockAdminActor(users, 42)
		key := "33333333-3333-4333-8333-333333333333"
		fingerprint := intentHash("admin.moment.moderate", struct {
			MomentID string `json:"momentId"`
			Action   string `json:"action"`
		}{MomentID: "11111111-1111-4111-8111-111111111111", Action: "deny_points"})
		media.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&mediaEntities.Operation{Operation: "admin.moment.moderate", IntentHash: fingerprint, State: "completed"}, nil).Once()
		moments.On("FindMoment", mock.Anything, "11111111-1111-4111-8111-111111111111", uint64(42), false).
			Return(nil, errors.New("connection reset")).Once()
		_, err := service.Moderate(ctx, "11111111-1111-4111-8111-111111111111", key, &messages.ModerationRequestDTO{Action: "deny_points"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("moderation decision persistence failure is redacted", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockAdminActor(users, 42)
		key := "33333333-3333-4333-8333-333333333333"
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		moment := &momentEntities.Moment{ID: "moment-1", RewardStatus: momentEntities.RewardAwarded, PointsAwarded: 10}
		asset := &mediaEntities.Asset{ID: "asset-1", State: mediaEntities.AssetAvailable}
		moments.On("ApplyModeration", mock.Anything, "11111111-1111-4111-8111-111111111111", "deny_points", uint64(42), key, mock.Anything).
			Return(moment, asset, true, nil).Once()
		moments.On("CreateModerationDecision", mock.Anything, mock.AnythingOfType("*entities.ModerationDecision")).
			Return(false, errors.New("connection reset")).Once()
		_, err := service.Moderate(ctx, "11111111-1111-4111-8111-111111111111", key, &messages.ModerationRequestDTO{Action: "deny_points"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("a durable no-op decision is not re-audited", func(t *testing.T) {
		service, moments, media, users, _ := newMomentServiceWithMocks(t)
		mockAdminActor(users, 42)
		key := "33333333-3333-4333-8333-333333333333"
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		moment := &momentEntities.Moment{ID: "moment-1", RewardStatus: momentEntities.RewardReversed, PublicationStatus: momentEntities.PublicationPrivate, ModerationStatus: momentEntities.ModerationRejected}
		asset := &mediaEntities.Asset{ID: "asset-1", State: mediaEntities.AssetAvailable}
		moments.On("ApplyModeration", mock.Anything, "11111111-1111-4111-8111-111111111111", "deny_points", uint64(42), key, mock.Anything).
			Return(moment, asset, false, nil).Once()
		media.On("CreateOperation", mock.Anything, mock.AnythingOfType("*entities.Operation")).Return(nil).Once()
		response, err := service.Moderate(ctx, "11111111-1111-4111-8111-111111111111", key, &messages.ModerationRequestDTO{Action: "deny_points"})
		require.NoError(t, err)
		assert.Equal(t, "moment-1", response.MomentID)
	})

	t.Run("audit write failure is redacted", func(t *testing.T) {
		service, moments, media, users, audits := newMomentServiceWithMocks(t)
		mockAdminActor(users, 42)
		key := "33333333-3333-4333-8333-333333333333"
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		moment := &momentEntities.Moment{ID: "moment-1", RewardStatus: momentEntities.RewardAwarded, PointsAwarded: 10}
		asset := &mediaEntities.Asset{ID: "asset-1", State: mediaEntities.AssetAvailable}
		moments.On("ApplyModeration", mock.Anything, "11111111-1111-4111-8111-111111111111", "deny_points", uint64(42), key, mock.Anything).
			Return(moment, asset, true, nil).Once()
		moments.On("CreateModerationDecision", mock.Anything, mock.AnythingOfType("*entities.ModerationDecision")).Return(true, nil).Once()
		audits.On("Create", mock.Anything, mock.Anything).Return(nil, errors.New("connection reset")).Once()
		_, err := service.Moderate(ctx, "11111111-1111-4111-8111-111111111111", key, &messages.ModerationRequestDTO{Action: "deny_points"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("delete_photo cleanup enqueue failure is redacted", func(t *testing.T) {
		service, moments, media, users, audits := newMomentServiceWithMocks(t)
		mockAdminActor(users, 42)
		key := "33333333-3333-4333-8333-333333333333"
		media.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		media.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(false, nil).Once()
		moment := &momentEntities.Moment{ID: "moment-1", RewardStatus: momentEntities.RewardNotApplicable}
		asset := &mediaEntities.Asset{ID: "asset-1", State: mediaEntities.AssetDeleted}
		moments.On("ApplyModeration", mock.Anything, "11111111-1111-4111-8111-111111111111", "delete_photo", uint64(42), key, mock.Anything).
			Return(moment, asset, true, nil).Once()
		moments.On("CreateModerationDecision", mock.Anything, mock.AnythingOfType("*entities.ModerationDecision")).Return(true, nil).Once()
		audits.On("Create", mock.Anything, mock.Anything).Return(nil, nil).Once()
		media.On("CreateCleanupJob", mock.Anything, mock.AnythingOfType("*entities.CleanupJob")).
			Return(false, errors.New("connection reset")).Once()
		_, err := service.Moderate(ctx, "11111111-1111-4111-8111-111111111111", key, &messages.ModerationRequestDTO{Action: "delete_photo"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
