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

func TestIteration4Routes_PublicSpacesAndProtectedOperations(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("API_PREFIX", "/v1")
	t.Setenv("JWT_IDENTITY_SECRET", "iteration4-router-secret")
	spaces := mocks.NewMockSpaceServiceInterface(t)
	spaces.On("List", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{InstallationHandler: &handlers.InstallationHandler{SpaceService: spaces}})
	router.RegisterInstallationRoutes()

	// when
	public := httptest.NewRecorder()
	engine.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/v2/spaces", nil))
	start := httptest.NewRecorder()
	engine.ServeHTTP(start, httptest.NewRequest(http.MethodPost, "/v2/manager/activities/11111111-1111-4111-8111-111111111111/start", nil))
	pause := httptest.NewRecorder()
	engine.ServeHTTP(pause, httptest.NewRequest(http.MethodPost, "/v2/manager/activities/11111111-1111-4111-8111-111111111111/pause", nil))

	// then
	assert.NotEqual(t, http.StatusUnauthorized, public.Code)
	assert.Equal(t, http.StatusUnauthorized, start.Code)
	assert.Equal(t, http.StatusUnauthorized, pause.Code)
	assert.Contains(t, start.Body.String(), "UNAUTHENTICATED")
	assert.Contains(t, pause.Body.String(), "UNAUTHENTICATED")
}
