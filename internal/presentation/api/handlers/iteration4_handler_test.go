package handlers_test

import (
	"net/http"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func iteration4Engine(t *testing.T, spaces *mocks.MockSpaceServiceInterface, activities *mocks.MockActivityServiceInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := &handlers.InstallationHandler{SpaceService: spaces, ActivityService: activities}
	engine.GET("/v2/spaces", handler.ListSpaces)
	engine.POST("/v2/manager/activities/:id/start", handler.StartActivity)
	engine.POST("/v2/manager/activities/:id/pause", handler.PauseActivity)
	return engine
}

func TestIteration4Handlers_SpacesAndActivityOperations(t *testing.T) {
	// given
	spaces := mocks.NewMockSpaceServiceInterface(t)
	activities := mocks.NewMockActivityServiceInterface(t)
	mapReference := "map:capela"
	spaces.On("List", mock.Anything, mock.Anything).Return(&messages.PaginatedResponse[messages.SpaceResponseDTO]{Data: []messages.SpaceResponseDTO{{ID: "11111111-1111-4111-8111-111111111111", Name: "Capela", Slug: "capela", MapReference: &mapReference}}, Pagination: messages.Pagination{CurrentPage: 1, Limit: 20}}, nil).Once()
	activities.On("Start", mock.Anything, "22222222-2222-4222-8222-222222222222", "").Return(&messages.ActivityStateResponseDTO{ID: "22222222-2222-4222-8222-222222222222", Status: "active"}, nil).Once()
	activities.On("Pause", mock.Anything, "22222222-2222-4222-8222-222222222222", "").Return(&messages.ActivityStateResponseDTO{ID: "22222222-2222-4222-8222-222222222222", Status: "paused"}, nil).Once()
	engine := iteration4Engine(t, spaces, activities)

	// when
	listed := performJSON(engine, http.MethodGet, "/v2/spaces?page=1", "")
	started := performJSON(engine, http.MethodPost, "/v2/manager/activities/22222222-2222-4222-8222-222222222222/start", "")
	paused := performJSON(engine, http.MethodPost, "/v2/manager/activities/22222222-2222-4222-8222-222222222222/pause", "")

	// then
	assert.Equal(t, http.StatusOK, listed.Code)
	assert.Equal(t, "20", listed.Header().Get("X-Limit"))
	assert.Contains(t, listed.Body.String(), "map:capela")
	assert.Equal(t, http.StatusOK, started.Code)
	assert.Contains(t, started.Body.String(), `"status":"active"`)
	assert.Equal(t, http.StatusOK, paused.Code)
	assert.Contains(t, paused.Body.String(), `"status":"paused"`)
}

func TestIteration4Handlers_AllPublishedOperationErrors(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		status int
		code   string
	}{
		{name: "invalid request", status: 400, code: "INVALID_REQUEST"},
		{name: "unauthenticated", status: 401, code: "UNAUTHENTICATED"},
		{name: "forbidden", status: 403, code: "FORBIDDEN"},
		{name: "not found", status: 404, code: "NOT_FOUND"},
		{name: "state conflict", status: 409, code: "ACTIVITY_STATE_CONFLICT"},
		{name: "idempotency conflict", status: 409, code: "IDEMPOTENCY_KEY_REUSED"},
		{name: "internal", status: 500, code: "INTERNAL_ERROR"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// given
			spaces := mocks.NewMockSpaceServiceInterface(t)
			activities := mocks.NewMockActivityServiceInterface(t)
			activities.On("Start", mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.NewAPIServiceError(testCase.status, testCase.code, "erro", nil)).Once()
			engine := iteration4Engine(t, spaces, activities)

			// when
			response := performJSON(engine, http.MethodPost, "/v2/manager/activities/22222222-2222-4222-8222-222222222222/start", "")

			// then
			assert.Equal(t, testCase.status, response.Code)
			assert.Contains(t, response.Body.String(), testCase.code)
		})
	}
}

func TestIteration4Handlers_SpacesInternalError(t *testing.T) {
	// given
	spaces := mocks.NewMockSpaceServiceInterface(t)
	activities := mocks.NewMockActivityServiceInterface(t)
	spaces.On("List", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	engine := iteration4Engine(t, spaces, activities)

	// when
	response := performJSON(engine, http.MethodGet, "/v2/spaces", "")

	// then
	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Contains(t, response.Body.String(), "INTERNAL_ERROR")
}
