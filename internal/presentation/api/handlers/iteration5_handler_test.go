package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func iteration5HandlerEngine(t *testing.T, content *mocks.MockContentServiceInterface, favorites *mocks.MockFavoriteServiceInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	contentHandler := &handlers.ContentHandler{ContentService: content}
	favoriteHandler := &handlers.FavoriteHandler{FavoriteService: favorites}
	engine.GET("/v2/schedule", contentHandler.Schedule)
	engine.GET("/v2/activities", contentHandler.ListActivities)
	engine.GET("/v2/activities/:activityId", contentHandler.GetActivity)
	engine.GET("/v2/users/me/favorites", favoriteHandler.List)
	engine.PUT("/v2/users/me/favorites/:activityId", favoriteHandler.Put)
	engine.DELETE("/v2/users/me/favorites/:activityId", favoriteHandler.Delete)
	return engine
}

func iteration5Request(engine http.Handler, method, path, key string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, nil)
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestIteration5Handlers_AllEndpointsAndPublishedQueries(t *testing.T) {
	// given
	content := mocks.NewMockContentServiceInterface(t)
	favorites := mocks.NewMockFavoriteServiceInterface(t)
	generatedAt := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	content.On("Schedule", mock.Anything, mock.MatchedBy(func(filter *messages.ListScheduleFilterDTO) bool {
		return filter.View == "home" && filter.Sector == "palco"
	})).Return(&messages.ScheduleResponseDTO{Items: []messages.ScheduleItemResponseDTO{}, GeneratedAt: generatedAt}, nil).Once()
	content.On("ListActivities", mock.Anything, mock.MatchedBy(func(filter *messages.ListPublicActivitiesFilterDTO) bool {
		return filter.Kind == "live" && filter.SpaceID != "" && filter.GetPage() == 1
	})).Return(&messages.PaginatedResponse[messages.PublicActivityResponseDTO]{Data: []messages.PublicActivityResponseDTO{}, Pagination: messages.Pagination{CurrentPage: 2, Limit: 10}}, nil).Once()
	content.On("GetActivity", mock.Anything, "11111111-1111-4111-8111-111111111111").Return(&messages.PublicActivityResponseDTO{ID: "11111111-1111-4111-8111-111111111111", Name: "Atividade"}, nil).Once()
	favorites.On("List", mock.Anything, mock.MatchedBy(func(filter *messages.ListFavoritesFilterDTO) bool { return filter.GetPage() == 0 })).Return(&messages.PaginatedResponse[messages.PublicActivityResponseDTO]{Data: []messages.PublicActivityResponseDTO{}, Pagination: messages.Pagination{CurrentPage: 1, Limit: 10}}, nil).Once()
	favorites.On("Put", mock.Anything, "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222").Return(nil).Once()
	favorites.On("Delete", mock.Anything, "11111111-1111-4111-8111-111111111111", "33333333-3333-4333-8333-333333333333").Return(nil).Once()
	engine := iteration5HandlerEngine(t, content, favorites)

	// when
	schedule := iteration5Request(engine, http.MethodGet, "/v2/schedule?view=home&sector=palco", "")
	activities := iteration5Request(engine, http.MethodGet, "/v2/activities?kind=live&spaceId=11111111-1111-4111-8111-111111111111&page=2", "")
	detail := iteration5Request(engine, http.MethodGet, "/v2/activities/11111111-1111-4111-8111-111111111111", "")
	listed := iteration5Request(engine, http.MethodGet, "/v2/users/me/favorites?page=0", "")
	put := iteration5Request(engine, http.MethodPut, "/v2/users/me/favorites/11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222")
	deleted := iteration5Request(engine, http.MethodDelete, "/v2/users/me/favorites/11111111-1111-4111-8111-111111111111", "33333333-3333-4333-8333-333333333333")

	// then
	assert.Equal(t, http.StatusOK, schedule.Code)
	assert.Contains(t, schedule.Body.String(), "generatedAt")
	assert.Equal(t, http.StatusOK, activities.Code)
	assert.Contains(t, activities.Body.String(), "pagination")
	assert.Equal(t, http.StatusOK, detail.Code)
	assert.Equal(t, http.StatusOK, listed.Code)
	assert.Equal(t, http.StatusNoContent, put.Code)
	assert.Empty(t, put.Body.String())
	assert.Equal(t, http.StatusNoContent, deleted.Code)
}

func TestIteration5Handlers_RejectUnknownRepeatedAndInvalidPageQueries(t *testing.T) {
	// given
	content := mocks.NewMockContentServiceInterface(t)
	favorites := mocks.NewMockFavoriteServiceInterface(t)
	engine := iteration5HandlerEngine(t, content, favorites)
	massAssignment := httptest.NewRecorder()
	massAssignmentRequest := httptest.NewRequest(http.MethodPut, "/v2/users/me/favorites/11111111-1111-4111-8111-111111111111", strings.NewReader(`{"userId":"9"}`))
	massAssignmentRequest.Header.Set("Idempotency-Key", "22222222-2222-4222-8222-222222222222")
	engine.ServeHTTP(massAssignment, massAssignmentRequest)

	// when
	responses := []*httptest.ResponseRecorder{
		iteration5Request(engine, http.MethodGet, "/v2/schedule?admin=true", ""),
		iteration5Request(engine, http.MethodGet, "/v2/schedule?view=home&view=home", ""),
		iteration5Request(engine, http.MethodGet, "/v2/activities?page=-1", ""),
		iteration5Request(engine, http.MethodGet, "/v2/activities?page=1000001", ""),
		iteration5Request(engine, http.MethodGet, "/v2/activities/11111111-1111-4111-8111-111111111111?include=assignments", ""),
		iteration5Request(engine, http.MethodGet, "/v2/users/me/favorites?page=abc", ""),
		iteration5Request(engine, http.MethodPut, "/v2/users/me/favorites/11111111-1111-4111-8111-111111111111?userId=9", ""),
		iteration5Request(engine, http.MethodPut, "/v2/users/me/favorites/11111111-1111-4111-8111-111111111111", ""),
		massAssignment,
	}

	// then
	for _, response := range responses {
		assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "INVALID_REQUEST")
	}
}

func TestIteration5Handlers_MapsEveryPublishedServiceStatus(t *testing.T) {
	for _, testCase := range []struct {
		status int
		code   string
	}{
		{400, "INVALID_REQUEST"}, {401, "UNAUTHENTICATED"}, {404, "NOT_FOUND"}, {409, "IDEMPOTENCY_KEY_REUSED"}, {500, "INTERNAL_ERROR"},
	} {
		t.Run(strings.ToLower(testCase.code), func(t *testing.T) {
			// given
			content := mocks.NewMockContentServiceInterface(t)
			favorites := mocks.NewMockFavoriteServiceInterface(t)
			favorites.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(appErrors.NewAPIServiceError(testCase.status, testCase.code, "erro", nil)).Once()
			engine := iteration5HandlerEngine(t, content, favorites)

			// when
			response := iteration5Request(engine, http.MethodPut, "/v2/users/me/favorites/11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222")

			// then
			assert.Equal(t, testCase.status, response.Code)
			assert.Contains(t, response.Body.String(), testCase.code)
		})
	}
}

func TestIteration5Handlers_ContentAndFavoriteListFailures(t *testing.T) {
	// given
	content := mocks.NewMockContentServiceInterface(t)
	favorites := mocks.NewMockFavoriteServiceInterface(t)
	content.On("Schedule", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	content.On("ListActivities", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	content.On("GetActivity", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	favorites.On("List", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	engine := iteration5HandlerEngine(t, content, favorites)

	// when
	responses := []*httptest.ResponseRecorder{
		iteration5Request(engine, http.MethodGet, "/v2/schedule", ""),
		iteration5Request(engine, http.MethodGet, "/v2/activities", ""),
		iteration5Request(engine, http.MethodGet, "/v2/activities/11111111-1111-4111-8111-111111111111", ""),
		iteration5Request(engine, http.MethodGet, "/v2/users/me/favorites", ""),
	}

	// then
	for _, response := range responses {
		assert.Equal(t, http.StatusInternalServerError, response.Code)
		assert.Contains(t, response.Body.String(), "INTERNAL_ERROR")
	}
}
