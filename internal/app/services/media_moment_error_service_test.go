package services

import (
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func mockMomentService(
	t *testing.T,
	moments *mocks.MockMomentRepository,
	media *mocks.MockMediaRepository,
	storage *mocks.MockMediaStorage,
	users *mocks.MockUserRepositoryInterface,
) *MomentService {
	t.Helper()
	audits := mocks.NewMockOperationAuditRepositoryInterface(t)
	service := NewMomentService(TestSuite.BaseService, moments, media, storage, users, audits).(*MomentService)
	service.now = func() time.Time { return mediaMomentNow }
	service.cursorSecret = func() string { return "cursor-test-secret" }
	return service
}

func defaultMediaUser() *userEntities.User {
	return &userEntities.User{ID: 42, Role: userEntities.RoleDefault, OnboardingComplete: true}
}

func adminMediaUser() *userEntities.User {
	return &userEntities.User{ID: 42, Role: userEntities.RoleAdmin, OnboardingComplete: true}
}

func visibleMoment() *momentEntities.Moment {
	return &momentEntities.Moment{
		ID:                  uuid.NewString(),
		MediaAssetID:        uuid.NewString(),
		PublicationStatus:   momentEntities.PublicationPublic,
		ModerationStatus:    momentEntities.ModerationApproved,
		AssetAvailable:      true,
		AuthorEligible:      true,
		AssetRetentionDueAt: mediaMomentNow.Add(time.Hour),
		CapturedAt:          mediaMomentNow,
	}
}

func TestMediaMoments_ServiceDependencyFailuresAreRedacted(t *testing.T) {
	TestSuite.DefaultSetup(t)
	ctx := TestSuite.ContextWithUser(42)
	databaseErr := errors.New("database details")
	providerErr := errors.New("provider details")

	t.Run("list repository failure", func(t *testing.T) {
		moments := mocks.NewMockMomentRepository(t)
		media := mocks.NewMockMediaRepository(t)
		storage := mocks.NewMockMediaStorage(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
		moments.On("ListMoments", mock.Anything, "feed", uint64(42), (*uint64)(nil), (*momentEntities.Cursor)(nil), mediaMomentNow).
			Return(nil, databaseErr)
		_, err := mockMomentService(t, moments, media, storage, users).List(ctx, "feed", "")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("list mapping database and storage failures", func(t *testing.T) {
		for name, mediaResult := range map[string]struct {
			asset *mediaEntities.Asset
			err   error
		}{
			"database": {nil, databaseErr},
			"storage":  {&mediaEntities.Asset{}, nil},
		} {
			t.Run(name, func(t *testing.T) {
				moments := mocks.NewMockMomentRepository(t)
				media := mocks.NewMockMediaRepository(t)
				storage := mocks.NewMockMediaStorage(t)
				users := mocks.NewMockUserRepositoryInterface(t)
				item := visibleMoment()
				users.On("FindByID", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
				moments.On("ListMoments", mock.Anything, "feed", uint64(42), (*uint64)(nil), (*momentEntities.Cursor)(nil), mediaMomentNow).
					Return(&momentEntities.Page{Items: []momentEntities.Moment{*item}}, nil)
				media.On("FindAsset", mock.Anything, item.MediaAssetID, false).Return(mediaResult.asset, mediaResult.err)
				if name == "storage" {
					storage.On("PresignDownload", mock.Anything, mediaResult.asset, mediaMomentNow, mediaReadLifetime).
						Return("", providerErr)
				}
				_, err := mockMomentService(t, moments, media, storage, users).List(ctx, "feed", "")
				if name == "database" {
					assert.ErrorIs(t, err, appErrors.InternalError)
				} else {
					assertAPIErrorCode(t, err, "MEDIA_UNAVAILABLE")
				}
			})
		}
	})

	t.Run("cursor configuration failure", func(t *testing.T) {
		moments := mocks.NewMockMomentRepository(t)
		media := mocks.NewMockMediaRepository(t)
		storage := mocks.NewMockMediaStorage(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		item := visibleMoment()
		item.AssetAvailable = false
		users.On("FindByID", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
		moments.On("ListMoments", mock.Anything, "feed", uint64(42), (*uint64)(nil), (*momentEntities.Cursor)(nil), mediaMomentNow).
			Return(&momentEntities.Page{Items: []momentEntities.Moment{*item}, HasNext: true}, nil)
		service := mockMomentService(t, moments, media, storage, users)
		service.cursorSecret = func() string { return "" }
		_, err := service.List(ctx, "feed", "")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("like mutation failure", func(t *testing.T) {
		moments := mocks.NewMockMomentRepository(t)
		media := mocks.NewMockMediaRepository(t)
		storage := mocks.NewMockMediaStorage(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		item := visibleMoment()
		users.On("FindByID", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
		moments.On("FindMoment", mock.Anything, item.ID, uint64(42), true).Return(item, nil)
		media.On("FindOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound)
		media.On("FindLegacyOperation", mock.Anything, uint64(42), mock.Anything).Return(false, nil)
		moments.On("ToggleLike", mock.Anything, item.ID, uint64(42), mediaMomentNow).
			Return(false, 0, databaseErr)
		_, err := mockMomentService(t, moments, media, storage, users).ToggleLike(ctx, item.ID, uuid.NewString())
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("moderation repository errors", func(t *testing.T) {
		for name, result := range map[string]struct {
			err  error
			code string
		}{
			"missing":  {appErrors.ErrNotFound, "NOT_FOUND"},
			"conflict": {appErrors.ErrConflict, "MODERATION_ACTION_INVALID"},
			"database": {databaseErr, ""},
		} {
			t.Run(name, func(t *testing.T) {
				moments := mocks.NewMockMomentRepository(t)
				media := mocks.NewMockMediaRepository(t)
				storage := mocks.NewMockMediaStorage(t)
				users := mocks.NewMockUserRepositoryInterface(t)
				users.On("FindByID", mock.Anything, uint64(42)).Return(adminMediaUser(), nil).Twice()
				media.On("FindOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound)
				media.On("FindLegacyOperation", mock.Anything, uint64(42), mock.Anything).Return(false, nil)
				moments.On("ApplyModeration", mock.Anything, mock.Anything, "delete_photo", uint64(42), mock.Anything, mediaMomentNow).
					Return(nil, nil, false, result.err)
				_, err := mockMomentService(t, moments, media, storage, users).Moderate(
					ctx,
					uuid.NewString(),
					uuid.NewString(),
					&messages.ModerationRequestDTO{Action: "delete_photo"},
				)
				if result.code == "" {
					assert.ErrorIs(t, err, appErrors.InternalError)
				} else {
					assertAPIErrorCode(t, err, result.code)
				}
			})
		}
	})

	t.Run("moderation queue repository and provider failures", func(t *testing.T) {
		for name, result := range map[string]struct {
			item      *momentEntities.Moment
			listErr   error
			asset     *mediaEntities.Asset
			assetErr  error
			signedErr error
		}{
			"query":    {listErr: databaseErr},
			"asset":    {item: visibleMoment(), assetErr: databaseErr},
			"provider": {item: visibleMoment(), asset: &mediaEntities.Asset{}, signedErr: providerErr},
		} {
			t.Run(name, func(t *testing.T) {
				moments := mocks.NewMockMomentRepository(t)
				media := mocks.NewMockMediaRepository(t)
				storage := mocks.NewMockMediaStorage(t)
				users := mocks.NewMockUserRepositoryInterface(t)
				users.On("FindByID", mock.Anything, uint64(42)).Return(adminMediaUser(), nil)
				var items []momentEntities.Moment
				if result.item != nil {
					items = []momentEntities.Moment{*result.item}
				}
				moments.On("ListModeration", mock.Anything, "general", uint64(0), mediaMomentNow).
					Return(&momentEntities.ModerationPage{Items: items}, result.listErr)
				if result.item != nil {
					media.On("FindAsset", mock.Anything, result.item.MediaAssetID, false).
						Return(result.asset, result.assetErr)
				}
				if result.asset != nil {
					storage.On("PresignDownload", mock.Anything, result.asset, mediaMomentNow, mediaReadLifetime).
						Return("", result.signedErr)
				}
				_, err := mockMomentService(t, moments, media, storage, users).ListModeration(ctx, "general", 0)
				if name == "query" || name == "asset" {
					assert.ErrorIs(t, err, appErrors.InternalError)
				} else {
					assertAPIErrorCode(t, err, "MEDIA_UNAVAILABLE")
				}
			})
		}
	})

	t.Run("moment creation repository failures", func(t *testing.T) {
		t.Run("asset query", func(t *testing.T) {
			moments := mocks.NewMockMomentRepository(t)
			media := mocks.NewMockMediaRepository(t)
			storage := mocks.NewMockMediaStorage(t)
			users := mocks.NewMockUserRepositoryInterface(t)
			users.On("FindByID", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
			media.On("FindAsset", mock.Anything, mock.Anything, true).Return(nil, databaseErr)
			_, _, err := mockMomentService(t, moments, media, storage, users).Create(
				ctx,
				uuid.NewString(),
				&messages.CreateMomentRequestDTO{MediaAssetID: uuid.NewString()},
			)
			assert.ErrorIs(t, err, appErrors.InternalError)
		})

		t.Run("participation and activity queries", func(t *testing.T) {
			for name, result := range map[string]struct {
				participation    *gameEntities.Participation
				participationErr error
				activityErr      error
			}{
				"participation": {participationErr: databaseErr},
				"activity": {
					participation: &gameEntities.Participation{ID: uuid.NewString(), UserID: 42, ActivityID: uuid.NewString()},
					activityErr:   databaseErr,
				},
			} {
				t.Run(name, func(t *testing.T) {
					moments := mocks.NewMockMomentRepository(t)
					media := mocks.NewMockMediaRepository(t)
					storage := mocks.NewMockMediaStorage(t)
					users := mocks.NewMockUserRepositoryInterface(t)
					assetID := uuid.NewString()
					participationID := uuid.NewString()
					users.On("FindByID", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
					media.On("FindAsset", mock.Anything, assetID, true).Return(&mediaEntities.Asset{
						ID: assetID, OwnerUserID: 42, State: mediaEntities.AssetAvailable,
						RetentionDueAt: mediaMomentNow.Add(time.Hour),
					}, nil)
					media.On("FindOperation", mock.Anything, uint64(42), mock.Anything).
						Return(nil, appErrors.ErrNotFound)
					media.On("FindLegacyOperation", mock.Anything, uint64(42), mock.Anything).Return(false, nil)
					moments.On("FindParticipationForUpdate", mock.Anything, participationID).
						Return(result.participation, result.participationErr)
					if result.participation != nil {
						moments.On("FindActivityForUpdate", mock.Anything, result.participation.ActivityID).
							Return("", false, (*time.Time)(nil), (*time.Time)(nil), 0, "", (*string)(nil), result.activityErr)
					}
					_, _, err := mockMomentService(t, moments, media, storage, users).Create(
						ctx,
						uuid.NewString(),
						&messages.CreateMomentRequestDTO{
							MediaAssetID: assetID, ParticipationID: &participationID,
						},
					)
					assert.ErrorIs(t, err, appErrors.InternalError)
				})
			}
		})

		for name, result := range map[string]struct {
			award            bool
			operationFailure bool
		}{
			"moment insert":           {},
			"challenge award":         {award: true},
			"idempotency persistence": {operationFailure: true},
		} {
			t.Run(name, func(t *testing.T) {
				moments := mocks.NewMockMomentRepository(t)
				media := mocks.NewMockMediaRepository(t)
				storage := mocks.NewMockMediaStorage(t)
				users := mocks.NewMockUserRepositoryInterface(t)
				assetID := uuid.NewString()
				users.On("FindByID", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
				users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(defaultMediaUser(), nil)
				media.On("FindAsset", mock.Anything, assetID, true).Return(&mediaEntities.Asset{
					ID: assetID, OwnerUserID: 42, State: mediaEntities.AssetAvailable,
					RetentionDueAt: mediaMomentNow.Add(time.Hour),
				}, nil)
				media.On("FindOperation", mock.Anything, uint64(42), mock.Anything).
					Return(nil, appErrors.ErrNotFound)
				media.On("FindLegacyOperation", mock.Anything, uint64(42), mock.Anything).Return(false, nil)
				request := &messages.CreateMomentRequestDTO{MediaAssetID: assetID, PublishConsent: result.award}
				if result.award {
					participationID := uuid.NewString()
					activityID := uuid.NewString()
					request.ParticipationID = &participationID
					moments.On("FindParticipationForUpdate", mock.Anything, participationID).
						Return(&gameEntities.Participation{
							ID: participationID, UserID: 42, ActivityID: activityID, CanShareMoment: true,
						}, nil)
					moments.On("FindActivityForUpdate", mock.Anything, activityID).
						Return("active", true, (*time.Time)(nil), (*time.Time)(nil), 10, "Challenge", (*string)(nil), nil)
				}
				if name == "moment insert" {
					moments.On("CreateMoment", mock.Anything, mock.Anything).Return(databaseErr)
				} else {
					moments.On("CreateMoment", mock.Anything, mock.Anything).Return(nil)
				}
				if result.award {
					moments.On("AwardMoment", mock.Anything, mock.Anything, uint64(42), mock.Anything, 10, mediaMomentNow).
						Return(databaseErr)
				}
				if result.operationFailure {
					media.On("CreateOperation", mock.Anything, mock.Anything).Return(databaseErr)
				}
				_, _, err := mockMomentService(t, moments, media, storage, users).Create(
					ctx,
					uuid.NewString(),
					request,
				)
				assert.ErrorIs(t, err, appErrors.InternalError)
			})
		}
	})
}
