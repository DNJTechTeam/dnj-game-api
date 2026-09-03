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

func TestIteration6Routes_PublicCatalogAndRankingProtectedWrites(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("API_PREFIX", "/v1")
	t.Setenv("JWT_IDENTITY_SECRET", "iteration6-router-secret")
	service := mocks.NewMockGameServiceInterface(t)
	service.On("ListGames", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("GetGame", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("Rankings", mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{GameHandler: &handlers.GameHandler{GameService: service}})
	router.RegisterGameRoutes()

	// when
	public := []struct{ method, path string }{
		{http.MethodGet, "/v2/games"},
		{http.MethodGet, "/v2/games/11111111-1111-4111-8111-111111111111"},
		{http.MethodGet, "/v2/rankings?scope=individual"},
	}
	for _, route := range public {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
		assert.NotEqual(t, http.StatusUnauthorized, response.Code)
	}
	protected := []struct{ method, path string }{
		{http.MethodGet, "/v2/game/overview"},
		{http.MethodGet, "/v2/activity-runs/current"},
		{http.MethodGet, "/v2/admin/activities/11111111-1111-4111-8111-111111111111/qr"},
		{http.MethodGet, "/v2/participations/current"},
		{http.MethodPost, "/v2/qr/validate"},
		{http.MethodGet, "/v2/manager/game-overview"},
		{http.MethodGet, "/v2/manager/game-overview"},
		{http.MethodGet, "/v2/manager/runs/11111111-1111-4111-8111-111111111111"},
		{http.MethodPost, "/v2/manager/runs"},
		{http.MethodPost, "/v2/manager/runs/11111111-1111-4111-8111-111111111111/qr"},
		{http.MethodPost, "/v2/manager/runs/11111111-1111-4111-8111-111111111111/start"},
		{http.MethodPost, "/v2/manager/runs/11111111-1111-4111-8111-111111111111/pause"},
		{http.MethodPost, "/v2/manager/runs/11111111-1111-4111-8111-111111111111/resume"},
		{http.MethodPost, "/v2/manager/runs/11111111-1111-4111-8111-111111111111/results"},
		{http.MethodPost, "/v2/manager/runs/11111111-1111-4111-8111-111111111111/cancel"},
	}
	for _, route := range protected {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
		assert.Equal(t, http.StatusUnauthorized, response.Code, route.path)
		assert.Contains(t, response.Body.String(), "UNAUTHENTICATED")
	}
}
