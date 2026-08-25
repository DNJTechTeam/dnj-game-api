package routers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAdminInstallationRoutes_AllRegisteredAndAuthenticationProtected(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_IDENTITY_SECRET", "admin-router-secret")
	t.Setenv("API_PREFIX", "/v1")
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{AdminInstallationHandler: &handlers.AdminInstallationHandler{}})
	router.RegisterAdminInstallationRoutes()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/v2/admin/spaces"},
		{http.MethodPost, "/v2/admin/spaces"},
		{http.MethodPatch, "/v2/admin/spaces/11111111-1111-4111-8111-111111111111"},
		{http.MethodGet, "/v2/admin/activities"},
		{http.MethodPost, "/v2/admin/activities"},
		{http.MethodPatch, "/v2/admin/activities/22222222-2222-4222-8222-222222222222"},
		{http.MethodGet, "/v2/admin/staff?role=EVENT_MANAGER"},
		{http.MethodPatch, "/v2/admin/users/7/role"},
		{http.MethodGet, "/v2/admin/activities/22222222-2222-4222-8222-222222222222/managers"},
		{http.MethodPut, "/v2/admin/activities/22222222-2222-4222-8222-222222222222/managers/7"},
		{http.MethodDelete, "/v2/admin/activities/22222222-2222-4222-8222-222222222222/managers/7"},
	}

	// when / then
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(route.method, route.path, nil))
			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "UNAUTHENTICATED")
		})
	}
}
