package services

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationsHTTP_MiddlewareHandlerServiceRepositoryAndDatabase(t *testing.T) {
	service := setupNotificationServiceTest(t)
	participant, _ := seedNotificationUser(t, "notif-http@example.com", userEntities.RoleDefault, true)
	admin, _ := seedNotificationUser(t, "notif-http-admin@example.com", userEntities.RoleAdmin, true)
	manager, _ := seedNotificationUser(t, "notif-http-manager@example.com", userEntities.RoleEventManager, true)
	jwt := NewJwtService(TestSuite.BaseService)
	participantToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, participant)
	require.NoError(t, err)
	adminToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, admin)
	require.NoError(t, err)
	managerToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, manager)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{
		NotificationHandler: &handlers.NotificationHandler{NotificationService: service},
	})
	router.RegisterNotificationRoutes()

	// GET /notifications/preferences without auth is rejected.
	unauth := adminHTTPRequest(engine, http.MethodGet, "/v2/notifications/preferences", "", "", "")
	assert.Equal(t, http.StatusUnauthorized, unauth.Code)

	// GET /notifications/preferences returns defaults.
	prefs := adminHTTPRequest(engine, http.MethodGet, "/v2/notifications/preferences", "", participantToken, "")
	require.Equal(t, http.StatusOK, prefs.Code, prefs.Body.String())
	var prefsBody messages.NotificationPreferencesResponseDTO
	require.NoError(t, json.Unmarshal(prefs.Body.Bytes(), &prefsBody))
	assert.True(t, prefsBody.MomentModerationEnabled)

	// PUT /notifications/preferences without an Idempotency-Key is rejected.
	badKey := adminHTTPRequest(
		engine, http.MethodPut, "/v2/notifications/preferences",
		`{"pointsEnabled":false}`, participantToken, "",
	)
	assert.Equal(t, http.StatusBadRequest, badKey.Code)

	// PUT /notifications/preferences updates the stored preference.
	updated := adminHTTPRequest(
		engine, http.MethodPut, "/v2/notifications/preferences",
		`{"pointsEnabled":false}`, participantToken, uuid.NewString(),
	)
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
	var updatedBody messages.NotificationPreferencesResponseDTO
	require.NoError(t, json.Unmarshal(updated.Body.Bytes(), &updatedBody))
	assert.False(t, updatedBody.PointsEnabled)

	// Sending an unknown field is rejected by strict decoding.
	unknownField := adminHTTPRequest(
		engine, http.MethodPut, "/v2/notifications/preferences",
		`{"momentModerationEnabled":false}`, participantToken, uuid.NewString(),
	)
	assert.Equal(t, http.StatusBadRequest, unknownField.Code)

	// Admin broadcast reaches the DEFAULT participant.
	sendResponse := adminHTTPRequest(
		engine, http.MethodPost, "/v2/admin/notifications",
		`{"title":"Aviso geral","body":"Manutenção agendada"}`, adminToken, uuid.NewString(),
	)
	require.Equal(t, http.StatusCreated, sendResponse.Code, sendResponse.Body.String())
	var sendBody messages.AdminSendNotificationResponseDTO
	require.NoError(t, json.Unmarshal(sendResponse.Body.Bytes(), &sendBody))
	assert.Equal(t, messages.Uint64StringFromUint64(1), sendBody.RecipientCount)
	assert.NotContains(t, sendResponse.Body.String(), "userId")

	// EVENT_MANAGER cannot send administrative notifications.
	managerSend := adminHTTPRequest(
		engine, http.MethodPost, "/v2/admin/notifications",
		`{"title":"Aviso","body":"Corpo"}`, managerToken, uuid.NewString(),
	)
	assert.Equal(t, http.StatusForbidden, managerSend.Code)

	// An unknown field in the admin send body is rejected by strict decoding.
	unknownAdminField := adminHTTPRequest(
		engine, http.MethodPost, "/v2/admin/notifications",
		`{"title":"T","body":"B","recipientIds":["1"]}`, adminToken, uuid.NewString(),
	)
	assert.Equal(t, http.StatusBadRequest, unknownAdminField.Code)

	// A non-DEFAULT actor cannot read their own preferences, list notifications, or mark one read.
	adminPrefs := adminHTTPRequest(engine, http.MethodGet, "/v2/notifications/preferences", "", adminToken, "")
	assert.Equal(t, http.StatusForbidden, adminPrefs.Code)
	adminList := adminHTTPRequest(engine, http.MethodGet, "/v2/notifications", "", adminToken, "")
	assert.Equal(t, http.StatusForbidden, adminList.Code)

	// POST /notifications/{id}/read with a non-empty body is rejected before reaching the service.
	nonEmptyBody := adminHTTPRequest(
		engine, http.MethodPost, "/v2/notifications/"+uuid.NewString()+"/read",
		`{"read":true}`, participantToken, uuid.NewString(),
	)
	assert.Equal(t, http.StatusBadRequest, nonEmptyBody.Code)

	// GET /notifications lists the broadcast with an unread badge.
	list := adminHTTPRequest(engine, http.MethodGet, "/v2/notifications", "", participantToken, "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	var listBody messages.NotificationListResponseDTO
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listBody))
	require.Len(t, listBody.Data, 1)
	assert.Equal(t, messages.Uint64StringFromUint64(1), listBody.UnreadCount)
	notificationID := listBody.Data[0].ID

	// POST /notifications/{id}/read marks it read.
	read := adminHTTPRequest(
		engine, http.MethodPost, "/v2/notifications/"+notificationID+"/read", "", participantToken, uuid.NewString(),
	)
	require.Equal(t, http.StatusOK, read.Code, read.Body.String())
	var readBody messages.NotificationResponseDTO
	require.NoError(t, json.Unmarshal(read.Body.Bytes(), &readBody))
	assert.Equal(t, "read", readBody.State)

	// Reading a nonexistent notification returns a uniform 404.
	missing := adminHTTPRequest(
		engine, http.MethodPost, "/v2/notifications/"+uuid.NewString()+"/read", "", participantToken, uuid.NewString(),
	)
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

func TestNotificationsHTTP_ConcurrentMarkReadWithSameKeyIsSingleEffect(t *testing.T) {
	service := setupNotificationServiceTest(t)
	participant, _ := seedNotificationUser(t, "notif-concurrent@example.com", userEntities.RoleDefault, true)
	jwt := NewJwtService(TestSuite.BaseService)
	token, err := jwt.GenerateIdentityToken(TestSuite.Ctx, participant)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{
		NotificationHandler: &handlers.NotificationHandler{NotificationService: service},
	})
	router.RegisterNotificationRoutes()

	notification := seedNotification(t, participant.ID, "points", "unread", notificationServiceNow)
	key := uuid.NewString()

	const attempts = 8
	var wg sync.WaitGroup
	codes := make([]int, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			response := adminHTTPRequest(
				engine, http.MethodPost, "/v2/notifications/"+notification.ID+"/read", "", token, key,
			)
			codes[index] = response.Code
		}(i)
	}
	wg.Wait()

	for _, code := range codes {
		assert.Equal(t, http.StatusOK, code)
	}
	var operationCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.IdempotencyOperation{}).
		Where("idempotency_key = ?", key).Count(&operationCount).Error)
	assert.Equal(t, int64(1), operationCount)
}
