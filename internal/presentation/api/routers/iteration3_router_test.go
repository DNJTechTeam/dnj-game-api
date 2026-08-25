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

func TestIteration3Routes_AreRegisteredAndProtected(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("API_PREFIX", "/v1")
	t.Setenv("JWT_IDENTITY_SECRET", "iteration3-router-secret")
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{ProfileHandler: &handlers.ProfileHandler{}, GroupHandler: &handlers.GroupHandler{}, GroupInviteHandler: &handlers.GroupInviteHandler{}})
	router.RegisterProfileRoutes()
	router.RegisterGroupRoutes()
	cases := []struct{ method, path string }{
		{http.MethodGet, "/v2/users/me"},
		{http.MethodPatch, "/v2/users/me"},
		{http.MethodPatch, "/v2/users/me/group"},
		{http.MethodPost, "/v2/users/me/group"},
		{http.MethodGet, "/v2/groups"},
		{http.MethodGet, "/v2/groups/me"},
		{http.MethodGet, "/v2/groups/me/members"},
		{http.MethodPost, "/v2/groups/invites/consume"},
		{http.MethodGet, "/v2/admin/groups/1/invites"},
		{http.MethodPost, "/v2/admin/groups/1/invites"},
		{http.MethodPost, "/v2/admin/groups/1/invites/2/renew"},
		{http.MethodDelete, "/v2/admin/groups/1/invites/2"},
	}

	for _, testCase := range cases {
		// when
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(testCase.method, testCase.path, nil))

		// then
		assert.Equal(t, http.StatusUnauthorized, recorder.Code, testCase.method+" "+testCase.path)
		assert.Contains(t, recorder.Body.String(), "UNAUTHENTICATED")
	}
}
