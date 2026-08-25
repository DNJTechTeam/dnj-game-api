package services

import (
	"errors"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newMediaServiceWithMocks(
	t *testing.T,
) (*MediaService, *mocks.MockMediaRepository, *mocks.MockMediaStorage, *mocks.MockUserRepositoryInterface) {
	t.Helper()
	TestSuite.DefaultSetup(t)
	repo := mocks.NewMockMediaRepository(t)
	storage := mocks.NewMockMediaStorage(t)
	users := mocks.NewMockUserRepositoryInterface(t)
	service := NewMediaService(TestSuite.BaseService, repo, storage, users).(*MediaService)
	return service, repo, storage, users
}

func mockDefaultActor(users *mocks.MockUserRepositoryInterface, id uint64) *userEntities.User {
	actor := &userEntities.User{ID: id, Role: userEntities.RoleDefault, OnboardingComplete: true}
	users.On("FindByID", mock.Anything, id).Return(actor, nil).Maybe()
	users.On("FindByIDForUpdate", mock.Anything, id).Return(actor, nil).Maybe()
	return actor
}

// TestMediaMoments_CreateUploadIntentRepositoryFailures exercises the internal repository
// failure branches of CreateUploadIntent that a real Postgres round-trip cannot reliably force:
// a generic (non-not-found) actor lookup failure inside the transaction, a stuck/never-completed
// prior idempotency operation left behind by a crashed request, and a generic failure persisting
// the new asset or its idempotency record. All must fail closed without leaking the underlying
// repository error.
func TestMediaMoments_CreateUploadIntentRepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)
	request := &messages.CreateUploadIntentRequestDTO{
		ContentType: "image/jpeg", Bytes: 100, ChecksumSHA256: checksum([]byte("payload")),
	}

	t.Run("locked actor lookup generic failure is redacted", func(t *testing.T) {
		service, _, _, users := newMediaServiceWithMocks(t)
		users.On("FindByID", mock.Anything, uint64(42)).
			Return(&userEntities.User{ID: 42, Role: userEntities.RoleDefault, OnboardingComplete: true}, nil).Once()
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.CreateUploadIntent(ctx, "22222222-2222-4222-8222-222222222222", request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("generic idempotency operation lookup failure is redacted", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		repo.On("FindOperation", mock.Anything, uint64(42), "22222222-2222-4222-8222-222222222222").
			Return(nil, errors.New("connection reset")).Once()
		_, _, err := service.CreateUploadIntent(ctx, "22222222-2222-4222-8222-222222222222", request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("stuck never-completed prior operation is a safe internal error", func(t *testing.T) {
		service, repo, _, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		hash := intentHash("media.upload-intent.create", request)
		repo.On("FindOperation", mock.Anything, uint64(42), "22222222-2222-4222-8222-222222222222").
			Return(&mediaEntities.Operation{
				ID: "op-1", ActorUserID: 42, IdempotencyKey: "22222222-2222-4222-8222-222222222222",
				Operation: "media.upload-intent.create", IntentHash: hash, State: "processing",
			}, nil).Once()
		_, _, err := service.CreateUploadIntent(ctx, "22222222-2222-4222-8222-222222222222", request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("CreateAsset generic failure is redacted", func(t *testing.T) {
		service, repo, storage, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", "2026-10-18T23:00:00-03:00")
		t.Setenv("S3_BUCKET", "dnj-private-test")
		storage.On("ValidateConfiguration").Return(nil).Once()
		repo.On("FindOperation", mock.Anything, uint64(42), "22222222-2222-4222-8222-222222222222").
			Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", mock.Anything, uint64(42), "22222222-2222-4222-8222-222222222222").
			Return(false, nil).Once()
		repo.On("CreateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).
			Return(errors.New("connection reset")).Once()
		_, _, err := service.CreateUploadIntent(ctx, "22222222-2222-4222-8222-222222222222", request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("idempotency operation persistence generic failure is redacted", func(t *testing.T) {
		service, repo, storage, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", "2026-10-18T23:00:00-03:00")
		t.Setenv("S3_BUCKET", "dnj-private-test")
		storage.On("ValidateConfiguration").Return(nil).Once()
		repo.On("FindOperation", mock.Anything, uint64(42), "22222222-2222-4222-8222-222222222222").
			Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", mock.Anything, uint64(42), "22222222-2222-4222-8222-222222222222").
			Return(false, nil).Once()
		repo.On("CreateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).Return(nil).Once()
		repo.On("CreateOperation", mock.Anything, mock.AnythingOfType("*entities.Operation")).
			Return(errors.New("connection reset")).Once()
		_, _, err := service.CreateUploadIntent(ctx, "22222222-2222-4222-8222-222222222222", request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("presign failure after a successful commit is reported as media unavailable", func(t *testing.T) {
		service, repo, storage, users := newMediaServiceWithMocks(t)
		mockDefaultActor(users, 42)
		t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", "2026-10-18T23:00:00-03:00")
		t.Setenv("S3_BUCKET", "dnj-private-test")
		storage.On("ValidateConfiguration").Return(nil).Once()
		repo.On("FindOperation", mock.Anything, uint64(42), "22222222-2222-4222-8222-222222222222").
			Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", mock.Anything, uint64(42), "22222222-2222-4222-8222-222222222222").
			Return(false, nil).Once()
		repo.On("CreateAsset", mock.Anything, mock.AnythingOfType("*entities.Asset")).Return(nil).Once()
		repo.On("CreateOperation", mock.Anything, mock.AnythingOfType("*entities.Operation")).Return(nil).Once()
		repo.On("CreateCleanupJob", mock.Anything, mock.AnythingOfType("*entities.CleanupJob")).Return(true, nil).Once()
		storage.On("PresignUpload", mock.Anything, mock.AnythingOfType("*entities.Asset"), mock.Anything).
			Return(nil, errors.New("s3 unreachable")).Once()
		_, _, err := service.CreateUploadIntent(ctx, "22222222-2222-4222-8222-222222222222", request)
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, "MEDIA_UNAVAILABLE", apiErr.Code)
	})
}
