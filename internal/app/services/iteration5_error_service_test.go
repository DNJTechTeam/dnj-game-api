package services

import (
	"context"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIteration5_ContentServiceRepositoryFailures(t *testing.T) {
	// given
	activities := mocks.NewMockActivityRepositoryInterface(t)
	spaces := mocks.NewMockSpaceRepositoryInterface(t)
	service := NewContentService(activities, spaces).(*ContentService)
	service.now = func() time.Time { return iteration5Now }
	activities.On("ListSchedule", mock.Anything, "", 100).Return(nil, appErrors.InternalError).Once()
	spaces.On("FindByID", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	activities.On("ListPublic", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	activities.On("FindPublicByID", mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()

	// when
	_, nilScheduleErr := service.Schedule(context.Background(), nil)
	_, scheduleErr := service.Schedule(context.Background(), &messages.ListScheduleFilterDTO{})
	_, nilListErr := service.ListActivities(context.Background(), nil)
	_, spaceErr := service.ListActivities(context.Background(), &messages.ListPublicActivitiesFilterDTO{SpaceID: uuid.NewString()})
	_, listErr := service.ListActivities(context.Background(), &messages.ListPublicActivitiesFilterDTO{})
	_, getErr := service.GetActivity(context.Background(), uuid.NewString())

	// then
	assert.Error(t, nilScheduleErr)
	assert.ErrorIs(t, scheduleErr, appErrors.InternalError)
	assert.Error(t, nilListErr)
	assert.ErrorIs(t, spaceErr, appErrors.InternalError)
	assert.ErrorIs(t, listErr, appErrors.InternalError)
	assert.ErrorIs(t, getErr, appErrors.InternalError)
}

func TestIteration5_FavoriteServiceRepositoryFailures(t *testing.T) {
	userID := uint64(42)
	ctx := TestSuite.ContextWithUser(userID)
	activityID := uuid.NewString()
	key := uuid.NewString()
	validUser := &userEntities.User{ID: userID, OnboardingComplete: true}

	t.Run("list failures", func(t *testing.T) {
		// given
		favorites := mocks.NewMockFavoriteRepositoryInterface(t)
		activities := mocks.NewMockActivityRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		service := NewFavoriteService(TestSuite.BaseService, favorites, activities, users).(*FavoriteService)
		users.On("FindByID", mock.Anything, userID).Return(nil, appErrors.InternalError).Once()
		users.On("FindByID", mock.Anything, userID).Return(validUser, nil).Once()
		favorites.On("ListVisible", mock.Anything, userID, mock.Anything, uint64(0)).Return(nil, appErrors.InternalError).Once()

		// when
		_, nilErr := service.List(ctx, nil)
		_, userErr := service.List(ctx, &messages.ListFavoritesFilterDTO{})
		_, repositoryErr := service.List(ctx, &messages.ListFavoritesFilterDTO{})

		// then
		assert.Error(t, nilErr)
		assert.ErrorIs(t, userErr, appErrors.InternalError)
		assert.ErrorIs(t, repositoryErr, appErrors.InternalError)
	})

	for _, testCase := range []struct {
		name      string
		operation string
		arrange   func(*mocks.MockFavoriteRepositoryInterface, *mocks.MockActivityRepositoryInterface)
	}{
		{"prior lookup", favoritePutOperation, func(f *mocks.MockFavoriteRepositoryInterface, _ *mocks.MockActivityRepositoryInterface) {
			f.On("FindOperation", mock.Anything, userID, key).Return(nil, appErrors.InternalError).Once()
		}},
		{"visibility lookup", favoritePutOperation, func(f *mocks.MockFavoriteRepositoryInterface, a *mocks.MockActivityRepositoryInterface) {
			f.On("FindOperation", mock.Anything, userID, key).Return(nil, appErrors.ErrNotFound).Once()
			a.On("FindPublicByID", mock.Anything, activityID, mock.Anything).Return(nil, appErrors.InternalError).Once()
		}},
		{"favorite create", favoritePutOperation, func(f *mocks.MockFavoriteRepositoryInterface, a *mocks.MockActivityRepositoryInterface) {
			f.On("FindOperation", mock.Anything, userID, key).Return(nil, appErrors.ErrNotFound).Once()
			a.On("FindPublicByID", mock.Anything, activityID, mock.Anything).Return(&activityEntities.PublicActivity{}, nil).Once()
			f.On("Create", mock.Anything, mock.Anything).Return(false, appErrors.InternalError).Once()
		}},
		{"favorite delete", favoriteDeleteOperation, func(f *mocks.MockFavoriteRepositoryInterface, _ *mocks.MockActivityRepositoryInterface) {
			f.On("FindOperation", mock.Anything, userID, key).Return(nil, appErrors.ErrNotFound).Once()
			f.On("Delete", mock.Anything, userID, activityID).Return(false, appErrors.InternalError).Once()
		}},
		{"operation create", favoriteDeleteOperation, func(f *mocks.MockFavoriteRepositoryInterface, _ *mocks.MockActivityRepositoryInterface) {
			f.On("FindOperation", mock.Anything, userID, key).Return(nil, appErrors.ErrNotFound).Once()
			f.On("Delete", mock.Anything, userID, activityID).Return(false, nil).Once()
			f.On("CreateOperation", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// given
			favorites := mocks.NewMockFavoriteRepositoryInterface(t)
			activities := mocks.NewMockActivityRepositoryInterface(t)
			users := mocks.NewMockUserRepositoryInterface(t)
			users.On("FindByIDForUpdate", mock.Anything, userID).Return(validUser, nil).Once()
			testCase.arrange(favorites, activities)
			service := NewFavoriteService(TestSuite.BaseService, favorites, activities, users).(*FavoriteService)

			// when
			err := service.write(ctx, activityID, key, testCase.operation)

			// then
			assert.ErrorIs(t, err, appErrors.InternalError)
		})
	}
}
