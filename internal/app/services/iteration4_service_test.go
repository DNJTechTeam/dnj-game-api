package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIteration4Test(t *testing.T) {
	t.Helper()
	TestSuite.DefaultSetup(t)
	for _, model := range []interface{ TableName() string }{
		&models.OperationAudit{}, &models.ActivityManagerAssignment{}, &models.Activity{}, &models.Space{}, &models.User{},
	} {
		TestSuite.TruncateTable(t, model)
	}
}

func seedIteration4User(t *testing.T, email string, role userEntities.UserRole) (*userEntities.User, context.Context) {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: email, Name: "Iteration Four User", MobilePhone: "41999999999", Role: role, OnboardingComplete: true})
	require.NoError(t, err)
	return user, TestSuite.ContextWithUser(user.ID)
}

func seedActivity(t *testing.T, status activityEntities.Status) string {
	t.Helper()
	id := uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: id, Slug: "activity-" + id, Name: "Radicalidade", Kind: string(activityEntities.KindCompetitive), Status: string(status), CheckInPoints: 10, MomentPoints: 20, CooldownSeconds: 60, AllowsMoment: true, CreatedAt: now, UpdatedAt: now}).Error)
	return id
}

func assignManager(t *testing.T, activityID string, userID uint64) {
	t.Helper()
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityManagerAssignment{ActivityID: activityID, UserID: userID, CreatedAt: time.Now().UTC()}).Error)
}

func newIteration4Services() (*SpaceService, *ActivityService) {
	return NewSpaceService(TestSuite.SpaceRepository).(*SpaceService), NewActivityService(TestSuite.BaseService, TestSuite.ActivityRepository, TestSuite.OperationAuditRepository, TestSuite.UserRepository).(*ActivityService)
}

func apiServiceError(t *testing.T, err error, status int, code string) {
	t.Helper()
	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, status, apiErr.Status)
	assert.Equal(t, code, apiErr.Code)
}

func TestSpaceService_ListDeterministicAndPaginated(t *testing.T) {
	// given
	setupIteration4Test(t)
	spaceService, _ := newIteration4Services()
	for index := 20; index >= 0; index-- {
		id := uuid.NewString()
		name := fmt.Sprintf("Space %02d", index)
		require.NoError(t, TestSuite.DbConn.Create(&models.Space{ID: id, Slug: fmt.Sprintf("space-%02d-%s", index, id), Name: name, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}).Error)
	}

	// when
	first, firstErr := spaceService.List(TestSuite.Ctx, &messages.ListSpacesFilterDTO{})
	secondFilter := &messages.ListSpacesFilterDTO{}
	secondFilter.SetPage(1)
	second, secondErr := spaceService.List(TestSuite.Ctx, secondFilter)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Len(t, first.Data, 20)
	assert.Equal(t, "Space 00", first.Data[0].Name)
	assert.Equal(t, "Space 19", first.Data[19].Name)
	assert.True(t, first.Pagination.HasNextPage)
	require.Len(t, second.Data, 1)
	assert.Equal(t, "Space 20", second.Data[0].Name)
	assert.False(t, second.Pagination.HasNextPage)
}

func TestActivityService_AdminGlobalStartPauseAndIdempotency(t *testing.T) {
	// given
	setupIteration4Test(t)
	_, activityService := newIteration4Services()
	_, adminCtx := seedIteration4User(t, "iteration4-admin@example.com", userEntities.RoleAdmin)
	activityID := seedActivity(t, activityEntities.StatusDraft)
	startKey := uuid.NewString()

	// when
	started, startErr := activityService.Start(adminCtx, activityID, startKey)
	retried, retryErr := activityService.Start(adminCtx, activityID, startKey)
	_, invalidStartErr := activityService.Start(adminCtx, activityID, uuid.NewString())
	paused, pauseErr := activityService.Pause(adminCtx, activityID, uuid.NewString())

	// then
	require.NoError(t, startErr)
	require.NoError(t, retryErr)
	require.NoError(t, pauseErr)
	assert.Equal(t, "active", started.Status)
	assert.Equal(t, started, retried)
	assert.Equal(t, "paused", paused.Status)
	apiServiceError(t, invalidStartErr, http.StatusConflict, "ACTIVITY_STATE_CONFLICT")
	var audits []models.OperationAudit
	require.NoError(t, TestSuite.DbConn.Order("created_at ASC").Find(&audits).Error)
	require.Len(t, audits, 2)
	assert.Equal(t, "activity.start", audits[0].Action)
	assert.JSONEq(t, `{"fromStatus":"draft","toStatus":"active"}`, string(audits[0].Metadata))
	assert.NotContains(t, string(audits[0].Metadata), "example.com")
}

func TestActivityService_ManagerAssignmentIsolationAndRoles(t *testing.T) {
	// given
	setupIteration4Test(t)
	_, activityService := newIteration4Services()
	manager, managerCtx := seedIteration4User(t, "iteration4-manager@example.com", userEntities.RoleEventManager)
	_, otherManagerCtx := seedIteration4User(t, "iteration4-other-manager@example.com", userEntities.RoleEventManager)
	defaultUser, defaultCtx := seedIteration4User(t, "iteration4-default@example.com", userEntities.RoleDefault)
	activityID := seedActivity(t, activityEntities.StatusDraft)
	assignManager(t, activityID, manager.ID)
	assignManager(t, activityID, defaultUser.ID)

	// when
	started, managerErr := activityService.Start(managerCtx, activityID, uuid.NewString())
	_, outsideErr := activityService.Pause(otherManagerCtx, activityID, uuid.NewString())
	_, missingErr := activityService.Pause(otherManagerCtx, uuid.NewString(), uuid.NewString())
	_, defaultErr := activityService.Pause(defaultCtx, activityID, uuid.NewString())

	// then
	require.NoError(t, managerErr)
	assert.Equal(t, "active", started.Status)
	apiServiceError(t, outsideErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, missingErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, defaultErr, http.StatusForbidden, "FORBIDDEN")
}

func TestActivityService_ValidatesIdentityIDsKeysAndDuplicateAssignments(t *testing.T) {
	// given
	setupIteration4Test(t)
	_, activityService := newIteration4Services()
	manager, managerCtx := seedIteration4User(t, "iteration4-validation@example.com", userEntities.RoleEventManager)
	activityID := seedActivity(t, activityEntities.StatusDraft)
	assignManager(t, activityID, manager.ID)

	// when
	_, missingIdentityErr := activityService.Start(context.Background(), activityID, uuid.NewString())
	_, malformedActivityErr := activityService.Start(managerCtx, "not-an-id", uuid.NewString())
	_, malformedKeyErr := activityService.Start(managerCtx, activityID, "not-a-key")
	duplicateErr := TestSuite.DbConn.Create(&models.ActivityManagerAssignment{ActivityID: activityID, UserID: manager.ID, CreatedAt: time.Now().UTC()}).Error

	// then
	apiServiceError(t, missingIdentityErr, http.StatusUnauthorized, "UNAUTHENTICATED")
	apiServiceError(t, malformedActivityErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, malformedKeyErr, http.StatusBadRequest, "INVALID_REQUEST")
	require.Error(t, duplicateErr)
}

func TestActivityService_ConcurrentTransitionsAreAtomic(t *testing.T) {
	// given
	setupIteration4Test(t)
	_, activityService := newIteration4Services()
	_, adminCtx := seedIteration4User(t, "iteration4-concurrency@example.com", userEntities.RoleAdmin)
	activityID := seedActivity(t, activityEntities.StatusDraft)
	const workers = 8
	errorsByWorker := make(chan error, workers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)

	// when
	for range workers {
		go func() {
			defer waitGroup.Done()
			_, err := activityService.Start(adminCtx, activityID, uuid.NewString())
			errorsByWorker <- err
		}()
	}
	waitGroup.Wait()
	close(errorsByWorker)

	// then
	successes := 0
	conflicts := 0
	for err := range errorsByWorker {
		if err == nil {
			successes++
			continue
		}
		var apiErr *appErrors.APIServiceError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict {
			conflicts++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, workers-1, conflicts)
	var auditCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).Where("action = ?", "activity.start").Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount)
}

func TestActivityService_ConcurrentRetryWithSameKeyHasOneEffect(t *testing.T) {
	// given
	setupIteration4Test(t)
	_, activityService := newIteration4Services()
	_, adminCtx := seedIteration4User(t, "iteration4-retry-concurrency@example.com", userEntities.RoleAdmin)
	activityID := seedActivity(t, activityEntities.StatusDraft)
	key := uuid.NewString()
	const workers = 6
	responses := make(chan *messages.ActivityStateResponseDTO, workers)
	errorsByWorker := make(chan error, workers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)

	// when
	for range workers {
		go func() {
			defer waitGroup.Done()
			response, err := activityService.Start(adminCtx, activityID, key)
			responses <- response
			errorsByWorker <- err
		}()
	}
	waitGroup.Wait()
	close(responses)
	close(errorsByWorker)

	// then
	for err := range errorsByWorker {
		require.NoError(t, err)
	}
	for response := range responses {
		require.NotNil(t, response)
		assert.Equal(t, activityID, response.ID)
		assert.Equal(t, "active", response.Status)
	}
	var auditCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).Where("actor_user_id IS NOT NULL AND idempotency_key = ?", key).Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount)
}

func TestIteration4HTTP_RealDatabaseAuthenticationAndPublicDiscovery(t *testing.T) {
	// given
	setupIteration4Test(t)
	spaceService, activityService := newIteration4Services()
	manager, _ := seedIteration4User(t, "iteration4-http@example.com", userEntities.RoleEventManager)
	activityID := seedActivity(t, activityEntities.StatusDraft)
	assignManager(t, activityID, manager.ID)
	spaceID := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Space{ID: spaceID, Slug: "capela", Name: "Capela", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}).Error)
	jwt := NewJwtService(TestSuite.BaseService)
	token, err := jwt.GenerateIdentityToken(TestSuite.Ctx, manager)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := &handlers.InstallationHandler{SpaceService: spaceService, ActivityService: activityService}
	router := routers.NewRouter(engine, &handlers.Handlers{InstallationHandler: handler})
	router.RegisterInstallationRoutes()

	// when
	spacesResponse := httptest.NewRecorder()
	engine.ServeHTTP(spacesResponse, httptest.NewRequest(http.MethodGet, "/v2/spaces", nil))
	unauthenticatedResponse := httptest.NewRecorder()
	engine.ServeHTTP(unauthenticatedResponse, httptest.NewRequest(http.MethodPost, "/v2/manager/activities/"+activityID+"/start", bytes.NewBuffer(nil)))
	startResponse := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/v2/manager/activities/"+activityID+"/start", bytes.NewBuffer(nil))
	startRequest.Header.Set("Authorization", "Bearer "+token)
	startRequest.Header.Set("Idempotency-Key", uuid.NewString())
	engine.ServeHTTP(startResponse, startRequest)

	// then
	assert.Equal(t, http.StatusOK, spacesResponse.Code)
	assert.JSONEq(t, `[{"id":"`+spaceID+`","name":"Capela","slug":"capela","mapReference":null}]`, spacesResponse.Body.String())
	assert.Equal(t, "1", spacesResponse.Header().Get("X-Page"))
	assert.Equal(t, http.StatusUnauthorized, unauthenticatedResponse.Code)
	assert.Contains(t, unauthenticatedResponse.Body.String(), "UNAUTHENTICATED")
	assert.Equal(t, http.StatusOK, startResponse.Code)
	assert.Contains(t, startResponse.Body.String(), `"status":"active"`)
}
