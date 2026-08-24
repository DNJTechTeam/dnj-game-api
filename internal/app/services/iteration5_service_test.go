package services

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var iteration5Now = time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)

func setupIteration5Test(t *testing.T) {
	t.Helper()
	TestSuite.DefaultSetup(t)
	for _, model := range []interface{ TableName() string }{
		&models.ParticipantOperation{}, &models.UserFavorite{}, &models.OperationAudit{}, &models.ActivityManagerAssignment{}, &models.Activity{}, &models.Space{}, &models.User{},
	} {
		TestSuite.TruncateTable(t, model)
	}
}

func seedIteration5User(t *testing.T, email string, onboarding bool) (*userEntities.User, context.Context) {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: email, Name: "Iteration Five User", MobilePhone: "41999999999", Role: userEntities.RoleDefault, OnboardingComplete: onboarding})
	require.NoError(t, err)
	return user, TestSuite.ContextWithUser(user.ID)
}

func seedIteration5Space(t *testing.T, name, slug string) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Space{ID: id, Name: name, Slug: slug, CreatedAt: iteration5Now, UpdatedAt: iteration5Now}).Error)
	return id
}

func seedIteration5Activity(t *testing.T, name string, kind activityEntities.Kind, status activityEntities.Status, spaceID *string, startsAt, endsAt *time.Time) string {
	t.Helper()
	id := uuid.NewString()
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "-" + strings.ReplaceAll(id[:8], "-", "")
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: id, SpaceID: spaceID, Slug: slug, Name: name, Description: stringPointer("Description " + name), Kind: string(kind), Status: string(status), StartsAt: startsAt, EndsAt: endsAt, CheckInPoints: 10, MomentPoints: 20, CooldownSeconds: 60, AllowsMoment: kind != activityEntities.KindSchedule, CreatedAt: iteration5Now, UpdatedAt: iteration5Now}).Error)
	return id
}

func stringPointer(value string) *string     { return &value }
func timePointer(value time.Time) *time.Time { return &value }

func newIteration5Services() (*ContentService, *FavoriteService) {
	content := NewContentService(TestSuite.ActivityRepository, TestSuite.SpaceRepository).(*ContentService)
	content.now = func() time.Time { return iteration5Now }
	favorites := NewFavoriteService(TestSuite.BaseService, TestSuite.FavoriteRepository, TestSuite.ActivityRepository, TestSuite.UserRepository).(*FavoriteService)
	favorites.now = func() time.Time { return iteration5Now }
	return content, favorites
}

func TestIteration5_DeriveActivityStateBoundaries(t *testing.T) {
	startsAt := iteration5Now
	endsAt := iteration5Now.Add(30 * time.Minute)
	cases := []struct {
		name   string
		status activityEntities.Status
		now    time.Time
		want   string
	}{
		{"before fifteen minutes", activityEntities.StatusDraft, startsAt.Add(-15*time.Minute - time.Nanosecond), "scheduled"},
		{"exactly fifteen minutes", activityEntities.StatusDraft, startsAt.Add(-15 * time.Minute), "upcoming"},
		{"exactly at start draft", activityEntities.StatusDraft, startsAt, "scheduled"},
		{"exactly at start active", activityEntities.StatusActive, startsAt, "live"},
		{"during active", activityEntities.StatusActive, startsAt.Add(time.Minute), "live"},
		{"paused during window", activityEntities.StatusPaused, startsAt.Add(time.Minute), "scheduled"},
		{"exactly at end", activityEntities.StatusActive, endsAt, "ended"},
		{"after end", activityEntities.StatusPaused, endsAt.Add(time.Nanosecond), "ended"},
		{"completed before end", activityEntities.StatusCompleted, startsAt.Add(-time.Hour), "ended"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// given
			activity := &activityEntities.Activity{Status: testCase.status, StartsAt: &startsAt, EndsAt: &endsAt}

			// when
			state := deriveActivityState(activity, testCase.now)

			// then
			require.NotNil(t, state)
			assert.Equal(t, testCase.want, *state)
		})
	}
	assert.Nil(t, deriveActivityState(&activityEntities.Activity{}, iteration5Now))
	invalidEnd := startsAt
	assert.Nil(t, deriveActivityState(&activityEntities.Activity{StartsAt: &startsAt, EndsAt: &invalidEnd}, iteration5Now))
}

func TestIteration5_ContentScheduleFullHomeSectorLimitAndUTC(t *testing.T) {
	// given
	setupIteration5Test(t)
	content, _ := newIteration5Services()
	spaceA := seedIteration5Space(t, "Palco", "palco")
	spaceB := seedIteration5Space(t, "Capela", "capela")
	liveStart := iteration5Now.Add(-time.Minute)
	liveEnd := iteration5Now.Add(time.Hour)
	seedIteration5Activity(t, "Live B", activityEntities.KindSchedule, activityEntities.StatusActive, &spaceB, &liveStart, &liveEnd)
	seedIteration5Activity(t, "Live A", activityEntities.KindSchedule, activityEntities.StatusActive, &spaceA, &liveStart, &liveEnd)
	for index := 0; index < 103; index++ {
		start := iteration5Now.Add(time.Duration(index+1) * time.Hour)
		end := start.Add(30 * time.Minute)
		seedIteration5Activity(t, fmt.Sprintf("Future %03d", index), activityEntities.KindSchedule, activityEntities.StatusDraft, &spaceA, &start, &end)
	}
	archivedStart := iteration5Now.Add(2 * time.Hour)
	archivedEnd := archivedStart.Add(time.Hour)
	seedIteration5Activity(t, "Archived", activityEntities.KindSchedule, activityEntities.StatusArchived, &spaceA, &archivedStart, &archivedEnd)
	seedIteration5Activity(t, "Invalid", activityEntities.KindSchedule, activityEntities.StatusDraft, &spaceA, nil, nil)
	seedIteration5Activity(t, "Not Schedule", activityEntities.KindLive, activityEntities.StatusActive, &spaceA, &liveStart, &liveEnd)

	// when
	full, fullErr := content.Schedule(TestSuite.Ctx, &messages.ListScheduleFilterDTO{})
	home, homeErr := content.Schedule(TestSuite.Ctx, &messages.ListScheduleFilterDTO{View: "home"})
	sector, sectorErr := content.Schedule(TestSuite.Ctx, &messages.ListScheduleFilterDTO{Sector: "capela"})
	_, badViewErr := content.Schedule(TestSuite.Ctx, &messages.ListScheduleFilterDTO{View: "all"})
	_, badSectorErr := content.Schedule(TestSuite.Ctx, &messages.ListScheduleFilterDTO{Sector: "Capela!"})

	// then
	require.NoError(t, fullErr)
	require.NoError(t, homeErr)
	require.NoError(t, sectorErr)
	assert.Len(t, full.Items, 100)
	assert.Equal(t, iteration5Now, full.GeneratedAt)
	assert.Equal(t, time.UTC, full.GeneratedAt.Location())
	require.Len(t, home.Items, 5)
	assert.Equal(t, []string{"Live A", "Live B", "Future 000", "Future 001", "Future 002"}, []string{home.Items[0].Title, home.Items[1].Title, home.Items[2].Title, home.Items[3].Title, home.Items[4].Title})
	assert.Equal(t, "live", home.Items[0].State)
	assert.Equal(t, "scheduled", home.Items[2].State)
	require.Len(t, sector.Items, 1)
	assert.Equal(t, "capela", sector.Items[0].Sector.Slug)
	apiServiceError(t, badViewErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, badSectorErr, http.StatusBadRequest, "INVALID_REQUEST")
}

func TestIteration5_PublicActivitiesVisibilityOrderingFiltersPaginationAndDetail(t *testing.T) {
	// given
	setupIteration5Test(t)
	content, _ := newIteration5Services()
	space := seedIteration5Space(t, "Palco", "palco")
	future := iteration5Now.Add(time.Hour)
	futureEnd := future.Add(time.Hour)
	draftScheduleID := seedIteration5Activity(t, "A Draft Schedule", activityEntities.KindSchedule, activityEntities.StatusDraft, &space, &future, &futureEnd)
	seedIteration5Activity(t, "Hidden Draft", activityEntities.KindCheckpoint, activityEntities.StatusDraft, &space, nil, nil)
	nullWindowID := seedIteration5Activity(t, "Z Active Null", activityEntities.KindCheckpoint, activityEntities.StatusActive, &space, nil, nil)
	past := iteration5Now.Add(-2 * time.Hour)
	pastEnd := iteration5Now.Add(-time.Hour)
	seedIteration5Activity(t, "B Paused", activityEntities.KindChallenge, activityEntities.StatusPaused, &space, &past, &pastEnd)
	completedID := seedIteration5Activity(t, "C Completed", activityEntities.KindCompetitive, activityEntities.StatusCompleted, nil, &past, &pastEnd)
	archivedID := seedIteration5Activity(t, "Archived", activityEntities.KindLive, activityEntities.StatusArchived, &space, &past, &pastEnd)
	for index := 0; index < 9; index++ {
		start := iteration5Now.Add(time.Duration(index+2) * time.Hour)
		end := start.Add(time.Hour)
		seedIteration5Activity(t, fmt.Sprintf("Schedule %02d", index), activityEntities.KindSchedule, activityEntities.StatusDraft, &space, &start, &end)
	}

	// when
	first, firstErr := content.ListActivities(TestSuite.Ctx, &messages.ListPublicActivitiesFilterDTO{})
	secondFilter := &messages.ListPublicActivitiesFilterDTO{}
	secondFilter.SetPage(1)
	second, secondErr := content.ListActivities(TestSuite.Ctx, secondFilter)
	kindResult, kindErr := content.ListActivities(TestSuite.Ctx, &messages.ListPublicActivitiesFilterDTO{Kind: "competitive"})
	spaceResult, spaceErr := content.ListActivities(TestSuite.Ctx, &messages.ListPublicActivitiesFilterDTO{SpaceID: space})
	detail, detailErr := content.GetActivity(TestSuite.Ctx, draftScheduleID)
	nullDetail, nullErr := content.GetActivity(TestSuite.Ctx, nullWindowID)
	completed, completedErr := content.GetActivity(TestSuite.Ctx, completedID)
	_, archivedErr := content.GetActivity(TestSuite.Ctx, archivedID)
	_, missingErr := content.GetActivity(TestSuite.Ctx, uuid.NewString())
	_, malformedErr := content.GetActivity(TestSuite.Ctx, "bad")
	_, kindValidationErr := content.ListActivities(TestSuite.Ctx, &messages.ListPublicActivitiesFilterDTO{Kind: "secret"})
	_, spaceValidationErr := content.ListActivities(TestSuite.Ctx, &messages.ListPublicActivitiesFilterDTO{SpaceID: "bad"})
	_, unknownSpaceErr := content.ListActivities(TestSuite.Ctx, &messages.ListPublicActivitiesFilterDTO{SpaceID: uuid.NewString()})

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, kindErr)
	require.NoError(t, spaceErr)
	assert.Len(t, first.Data, 10)
	assert.True(t, first.Pagination.HasNextPage)
	assert.Len(t, second.Data, 3)
	assert.False(t, second.Pagination.HasNextPage)
	assert.Equal(t, "Z Active Null", second.Data[len(second.Data)-1].Name)
	assert.Nil(t, second.Data[len(second.Data)-1].State)
	require.Len(t, kindResult.Data, 1)
	assert.Equal(t, completedID, kindResult.Data[0].ID)
	assert.Len(t, spaceResult.Data, 10)
	require.NoError(t, detailErr)
	require.NoError(t, nullErr)
	require.NoError(t, completedErr)
	assert.Equal(t, "scheduled", *detail.State)
	assert.Nil(t, nullDetail.State)
	assert.Equal(t, "ended", *completed.State)
	apiServiceError(t, archivedErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, missingErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, malformedErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, kindValidationErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, spaceValidationErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, unknownSpaceErr, http.StatusNotFound, "NOT_FOUND")
}

func TestIteration5_FavoritesAuthVisibilityIdempotencyNoEnumerationAndNoAudit(t *testing.T) {
	// given
	setupIteration5Test(t)
	_, favorites := newIteration5Services()
	user, ctx := seedIteration5User(t, "favorites@example.com", true)
	_, incompleteCtx := seedIteration5User(t, "incomplete@example.com", false)
	start := iteration5Now.Add(-time.Minute)
	end := iteration5Now.Add(time.Hour)
	activityID := seedIteration5Activity(t, "Favorite Activity", activityEntities.KindLive, activityEntities.StatusActive, nil, &start, &end)
	hiddenID := seedIteration5Activity(t, "Hidden Favorite", activityEntities.KindCheckpoint, activityEntities.StatusDraft, nil, nil, nil)
	putKey := uuid.NewString()

	// when
	putErr := favorites.Put(ctx, activityID, putKey)
	retryErr := favorites.Put(ctx, activityID, putKey)
	duplicateErr := favorites.Put(ctx, activityID, uuid.NewString())
	list, listErr := favorites.List(ctx, &messages.ListFavoritesFilterDTO{})
	reuseErr := favorites.Delete(ctx, activityID, putKey)
	hiddenErr := favorites.Put(ctx, hiddenID, uuid.NewString())
	missingPutErr := favorites.Put(ctx, uuid.NewString(), uuid.NewString())
	missingDeleteErr := favorites.Delete(ctx, uuid.NewString(), uuid.NewString())
	deleteKey := uuid.NewString()
	deleteErr := favorites.Delete(ctx, activityID, deleteKey)
	deleteRetryErr := favorites.Delete(ctx, activityID, deleteKey)
	_, unauthenticatedListErr := favorites.List(context.Background(), &messages.ListFavoritesFilterDTO{})
	_, incompleteListErr := favorites.List(incompleteCtx, &messages.ListFavoritesFilterDTO{})
	malformedKeyErr := favorites.Put(ctx, activityID, "bad")
	malformedActivityErr := favorites.Put(ctx, "bad", uuid.NewString())
	malformedDeleteErr := favorites.Delete(ctx, "bad", uuid.NewString())

	// then
	require.NoError(t, putErr)
	require.NoError(t, retryErr)
	require.NoError(t, duplicateErr)
	require.NoError(t, listErr)
	require.NoError(t, missingDeleteErr)
	require.NoError(t, deleteErr)
	require.NoError(t, deleteRetryErr)
	require.Len(t, list.Data, 1)
	assert.Equal(t, activityID, list.Data[0].ID)
	apiServiceError(t, reuseErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	apiServiceError(t, hiddenErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, missingPutErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, unauthenticatedListErr, http.StatusUnauthorized, "UNAUTHENTICATED")
	apiServiceError(t, incompleteListErr, http.StatusConflict, "ONBOARDING_REQUIRED")
	apiServiceError(t, malformedKeyErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, malformedActivityErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, malformedDeleteErr, http.StatusBadRequest, "INVALID_REQUEST")
	var favoriteCount, operationCount, auditCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.UserFavorite{}).Where("user_id = ?", user.ID).Count(&favoriteCount).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.ParticipantOperation{}).Where("actor_user_id = ?", user.ID).Count(&operationCount).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).Count(&auditCount).Error)
	assert.Zero(t, favoriteCount)
	assert.Equal(t, int64(4), operationCount)
	assert.Zero(t, auditCount)
}

func TestIteration5_FavoriteRetrySurvivesArchiveAndConcurrentKeysAreSafe(t *testing.T) {
	// given
	setupIteration5Test(t)
	_, favorites := newIteration5Services()
	user, ctx := seedIteration5User(t, "favorite-concurrency@example.com", true)
	start := iteration5Now.Add(-time.Minute)
	end := iteration5Now.Add(time.Hour)
	activityID := seedIteration5Activity(t, "Concurrent Favorite", activityEntities.KindLive, activityEntities.StatusActive, nil, &start, &end)
	key := uuid.NewString()
	require.NoError(t, favorites.Put(ctx, activityID, key))
	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("id = ?", activityID).Update("status", "archived").Error)

	// when
	archivedRetryErr := favorites.Put(ctx, activityID, key)
	afterArchive, listErr := favorites.List(ctx, &messages.ListFavoritesFilterDTO{})
	const workers = 8
	errorsByWorker := make(chan error, workers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)
	for range workers {
		go func() {
			defer waitGroup.Done()
			errorsByWorker <- favorites.Delete(ctx, uuid.NewString(), uuid.NewString())
		}()
	}
	waitGroup.Wait()
	close(errorsByWorker)

	// then
	require.NoError(t, archivedRetryErr)
	require.NoError(t, listErr)
	assert.Empty(t, afterArchive.Data)
	for err := range errorsByWorker {
		require.NoError(t, err)
	}
	var favoritesCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.UserFavorite{}).Where("user_id = ?", user.ID).Count(&favoritesCount).Error)
	assert.Equal(t, int64(1), favoritesCount)
}

func TestIteration5_FavoriteSameKeyConcurrencyAndDatabaseRevalidation(t *testing.T) {
	// given
	setupIteration5Test(t)
	_, favorites := newIteration5Services()
	user, ctx := seedIteration5User(t, "same-key@example.com", true)
	start := iteration5Now.Add(-time.Minute)
	end := iteration5Now.Add(time.Hour)
	activityID := seedIteration5Activity(t, "Same Key Favorite", activityEntities.KindLive, activityEntities.StatusActive, nil, &start, &end)
	key := uuid.NewString()
	const workers = 6
	errorsByWorker := make(chan error, workers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)

	// when
	for range workers {
		go func() {
			defer waitGroup.Done()
			errorsByWorker <- favorites.Put(ctx, activityID, key)
		}()
	}
	waitGroup.Wait()
	close(errorsByWorker)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", user.ID).Update("role", string(userEntities.RoleAdmin)).Error)
	roleChangedList, roleErr := favorites.List(ctx, &messages.ListFavoritesFilterDTO{})
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", user.ID).Update("onboarding_complete", false).Error)
	_, onboardingErr := favorites.List(ctx, &messages.ListFavoritesFilterDTO{})

	// then
	for err := range errorsByWorker {
		require.NoError(t, err)
	}
	require.NoError(t, roleErr)
	assert.Len(t, roleChangedList.Data, 1)
	apiServiceError(t, onboardingErr, http.StatusConflict, "ONBOARDING_REQUIRED")
	var favoriteCount, operationCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.UserFavorite{}).Count(&favoriteCount).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.ParticipantOperation{}).Count(&operationCount).Error)
	assert.Equal(t, int64(1), favoriteCount)
	assert.Equal(t, int64(1), operationCount)
}

func TestIteration5_ActivityOrderingIsStableForEqualTimestamps(t *testing.T) {
	// given
	setupIteration5Test(t)
	content, _ := newIteration5Services()
	start := iteration5Now.Add(time.Hour)
	end := start.Add(time.Hour)
	for _, name := range []string{"Bravo", "Alpha", "Alpha"} {
		seedIteration5Activity(t, name, activityEntities.KindSchedule, activityEntities.StatusDraft, nil, &start, &end)
	}

	// when
	result, err := content.ListActivities(TestSuite.Ctx, &messages.ListPublicActivitiesFilterDTO{})

	// then
	require.NoError(t, err)
	require.Len(t, result.Data, 3)
	names := []string{result.Data[0].Name, result.Data[1].Name, result.Data[2].Name}
	assert.True(t, sort.StringsAreSorted(names))
	if result.Data[0].Name == result.Data[1].Name {
		assert.Less(t, result.Data[0].ID, result.Data[1].ID)
	}
}

func TestIteration5_ContentNormalizesPositiveAndNegativeOffsetsToUTC(t *testing.T) {
	// given
	setupIteration5Test(t)
	content, _ := newIteration5Services()
	startsAt := iteration5Now.Add(-time.Minute).In(time.FixedZone("positive", 5*60*60+30*60))
	endsAt := iteration5Now.Add(time.Hour).In(time.FixedZone("negative", -7*60*60))
	activityID := seedIteration5Activity(t, "Offset Activity", activityEntities.KindLive, activityEntities.StatusActive, nil, &startsAt, &endsAt)

	// when
	detail, err := content.GetActivity(TestSuite.Ctx, activityID)

	// then
	require.NoError(t, err)
	require.NotNil(t, detail.StartsAt)
	require.NotNil(t, detail.EndsAt)
	assert.Equal(t, time.UTC, detail.StartsAt.Location())
	assert.Equal(t, time.UTC, detail.EndsAt.Location())
	assert.Equal(t, "2026-10-18T14:59:00Z", detail.StartsAt.Format(time.RFC3339))
	assert.Equal(t, "2026-10-18T16:00:00Z", detail.EndsAt.Format(time.RFC3339))
}
