package services

import (
	"net/http"
	"testing"
	"time"

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

func TestIteration5HTTP_MiddlewareHandlerServiceRepositoryAndDatabase(t *testing.T) {
	// given
	setupIteration5Test(t)
	content, favorites := newIteration5Services()
	user, _ := seedIteration5User(t, "iteration5-http@example.com", true)
	removed, _ := seedIteration5User(t, "iteration5-removed@example.com", true)
	jwt := NewJwtService(TestSuite.BaseService)
	token, err := jwt.GenerateIdentityToken(TestSuite.Ctx, user)
	require.NoError(t, err)
	removedToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, removed)
	require.NoError(t, err)
	require.NoError(t, TestSuite.DbConn.Delete(&models.User{}, removed.ID).Error)
	spaceID := seedIteration5Space(t, "Palco", "palco")
	start := iteration5Now.Add(-time.Minute)
	end := iteration5Now.Add(time.Hour)
	activityID := seedIteration5Activity(t, "HTTP Activity", activityEntities.KindSchedule, activityEntities.StatusActive, &spaceID, &start, &end)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{ContentHandler: &handlers.ContentHandler{ContentService: content}, FavoriteHandler: &handlers.FavoriteHandler{FavoriteService: favorites}})
	router.RegisterContentRoutes()

	// when
	schedule := adminHTTPRequest(engine, http.MethodGet, "/v2/schedule?view=home", "", "", "")
	activities := adminHTTPRequest(engine, http.MethodGet, "/v2/activities?kind=schedule&spaceId="+spaceID+"&page=1", "", "", "")
	detail := adminHTTPRequest(engine, http.MethodGet, "/v2/activities/"+activityID, "", "", "")
	unauthenticated := adminHTTPRequest(engine, http.MethodGet, "/v2/users/me/favorites", "", "", "")
	put := adminHTTPRequest(engine, http.MethodPut, "/v2/users/me/favorites/"+activityID, "", token, uuid.NewString())
	listed := adminHTTPRequest(engine, http.MethodGet, "/v2/users/me/favorites", "", token, "")
	removedUser := adminHTTPRequest(engine, http.MethodGet, "/v2/users/me/favorites", "", removedToken, "")
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", user.ID).Update("role", string(userEntities.RoleAdmin)).Error)
	roleChanged := adminHTTPRequest(engine, http.MethodGet, "/v2/users/me/favorites", "", token, "")
	deleted := adminHTTPRequest(engine, http.MethodDelete, "/v2/users/me/favorites/"+activityID, "", token, uuid.NewString())
	deletedAgain := adminHTTPRequest(engine, http.MethodDelete, "/v2/users/me/favorites/"+activityID, "", token, uuid.NewString())
	invalidQuery := adminHTTPRequest(engine, http.MethodGet, "/v2/activities?status=active", "", "", "")

	// then
	for _, response := range []int{schedule.Code, activities.Code, detail.Code, listed.Code, roleChanged.Code} {
		assert.Equal(t, http.StatusOK, response)
	}
	assert.Contains(t, schedule.Body.String(), `"state":"live"`)
	assert.Contains(t, schedule.Body.String(), `"generatedAt":"2026-10-18T15:00:00Z"`)
	assert.Contains(t, activities.Body.String(), `"data"`)
	assert.Contains(t, detail.Body.String(), `"checkInPoints":10`)
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	assert.Equal(t, http.StatusUnauthorized, removedUser.Code)
	assert.Equal(t, http.StatusNoContent, put.Code)
	assert.Equal(t, http.StatusNoContent, deleted.Code)
	assert.Equal(t, http.StatusNoContent, deletedAgain.Code)
	assert.Equal(t, http.StatusBadRequest, invalidQuery.Code)
}
