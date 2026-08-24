package routers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIteration5Routes_PublicContentAndProtectedFavorites(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("API_PREFIX", "/v1")
	t.Setenv("JWT_IDENTITY_SECRET", "iteration5-router-secret")
	content := mocks.NewMockContentServiceInterface(t)
	content.On("Schedule", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	content.On("ListActivities", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	content.On("GetActivity", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{ContentHandler: &handlers.ContentHandler{ContentService: content}, FavoriteHandler: &handlers.FavoriteHandler{FavoriteService: mocks.NewMockFavoriteServiceInterface(t)}})
	router.RegisterContentRoutes()

	// when
	schedule := httptest.NewRecorder()
	engine.ServeHTTP(schedule, httptest.NewRequest(http.MethodGet, "/v2/schedule", nil))
	activities := httptest.NewRecorder()
	engine.ServeHTTP(activities, httptest.NewRequest(http.MethodGet, "/v2/activities", nil))
	detail := httptest.NewRecorder()
	engine.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/v2/activities/11111111-1111-4111-8111-111111111111", nil))
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		favorite := httptest.NewRecorder()
		path := "/v2/users/me/favorites"
		if method != http.MethodGet {
			path += "/11111111-1111-4111-8111-111111111111"
		}
		engine.ServeHTTP(favorite, httptest.NewRequest(method, path, nil))
		assert.Equal(t, http.StatusUnauthorized, favorite.Code)
		assert.Contains(t, favorite.Body.String(), "UNAUTHENTICATED")
	}

	// then
	assert.NotEqual(t, http.StatusUnauthorized, schedule.Code)
	assert.NotEqual(t, http.StatusUnauthorized, activities.Code)
	assert.NotEqual(t, http.StatusUnauthorized, detail.Code)
}
