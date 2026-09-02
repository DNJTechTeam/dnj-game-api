package services

import (
	"encoding/json"
	"fmt"
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

	// Push configuration, subscription lifecycle and queue bridge all traverse
	// middleware/handler/service/repository with the real PostgreSQL schema.
	pushUnavailable := adminHTTPRequest(engine, http.MethodGet, "/v2/push/config", "", participantToken, "")
	t.Setenv("VAPID_PUBLIC_KEY", "test-vapid-public-key")
	t.Setenv("DNJ_QUEUE_BRIDGE_TOKEN", "queue-bridge-secret")
	pushConfig := adminHTTPRequest(engine, http.MethodGet, "/v2/push/config", "", participantToken, "")
	adminPushConfig := adminHTTPRequest(engine, http.MethodGet, "/v2/push/config", "", adminToken, "")

	const pushEndpoint = "https://push.example/subscriptions/http"
	pushBody := `{"endpoint":"` + pushEndpoint + `","p256dh":"key-1","auth":"auth-1"}`
	pushUnknownField := adminHTTPRequest(engine, http.MethodPut, "/v2/push/subscriptions", `{"endpoint":"`+pushEndpoint+`","p256dh":"key","auth":"auth","userId":"1"}`, participantToken, uuid.NewString())
	pushInvalidEndpoint := adminHTTPRequest(engine, http.MethodPut, "/v2/push/subscriptions", `{"endpoint":"http://push.example/sub","p256dh":"key","auth":"auth"}`, participantToken, uuid.NewString())
	pushInvalidKey := adminHTTPRequest(engine, http.MethodPut, "/v2/push/subscriptions", pushBody, participantToken, "not-a-uuid")
	pushForbidden := adminHTTPRequest(engine, http.MethodPut, "/v2/push/subscriptions", pushBody, adminToken, uuid.NewString())
	pushKey := uuid.NewString()
	pushCreated := adminHTTPRequest(engine, http.MethodPut, "/v2/push/subscriptions", pushBody, participantToken, pushKey)
	require.Equal(t, http.StatusOK, pushCreated.Code, pushCreated.Body.String())
	var pushCreatedBody messages.PushSubscriptionResponseDTO
	require.NoError(t, json.Unmarshal(pushCreated.Body.Bytes(), &pushCreatedBody))
	pushRetry := adminHTTPRequest(engine, http.MethodPut, "/v2/push/subscriptions", pushBody, participantToken, pushKey)
	pushKeyReused := adminHTTPRequest(engine, http.MethodPut, "/v2/push/subscriptions", `{"endpoint":"https://push.example/other","p256dh":"key","auth":"auth"}`, participantToken, pushKey)
	pushRotated := adminHTTPRequest(engine, http.MethodPut, "/v2/push/subscriptions", `{"endpoint":"`+pushEndpoint+`","p256dh":"key-2","auth":"auth-2"}`, participantToken, uuid.NewString())

	queueBody := fmt.Sprintf(`{"queueId":"pastoral","entryId":"entry-1","participantUserId":"%d","calledAt":"2026-08-25T12:00:00Z"}`, participant.ID)
	queueKey := "queue-called:pastoral:entry-1"
	queueMissingCredential := adminHTTPRequest(engine, http.MethodPost, "/v2/internal/notifications/queue-called", queueBody, "", queueKey)
	queueWrongCredential := adminHTTPRequest(engine, http.MethodPost, "/v2/internal/notifications/queue-called", queueBody, "wrong-secret", queueKey)
	queueUnknownField := adminHTTPRequest(engine, http.MethodPost, "/v2/internal/notifications/queue-called", fmt.Sprintf(`{"queueId":"pastoral","entryId":"entry-1","participantUserId":"%d","calledAt":"2026-08-25T12:00:00Z","actorId":"1"}`, participant.ID), "queue-bridge-secret", queueKey)
	queueInvalidKey := adminHTTPRequest(engine, http.MethodPost, "/v2/internal/notifications/queue-called", queueBody, "queue-bridge-secret", "wrong-key")
	queueInvalidRequest := adminHTTPRequest(engine, http.MethodPost, "/v2/internal/notifications/queue-called", `{"queueId":"pastoral","entryId":"invalid","participantUserId":"0","calledAt":"2026-08-25T12:00:00Z"}`, "queue-bridge-secret", "queue-called:pastoral:invalid")
	queueCreated := adminHTTPRequest(engine, http.MethodPost, "/v2/internal/notifications/queue-called", queueBody, "queue-bridge-secret", queueKey)
	require.Equal(t, http.StatusCreated, queueCreated.Code, queueCreated.Body.String())
	var queueCreatedBody messages.QueueCalledNotificationResponseDTO
	require.NoError(t, json.Unmarshal(queueCreated.Body.Bytes(), &queueCreatedBody))
	queueRetry := adminHTTPRequest(engine, http.MethodPost, "/v2/internal/notifications/queue-called", queueBody, "queue-bridge-secret", queueKey)

	deactivateBody := `{"endpoint":"` + pushEndpoint + `"}`
	deactivateUnknownField := adminHTTPRequest(engine, http.MethodDelete, "/v2/push/subscriptions", `{"endpoint":"`+pushEndpoint+`","userId":"1"}`, participantToken, uuid.NewString())
	deactivateInvalidEndpoint := adminHTTPRequest(engine, http.MethodDelete, "/v2/push/subscriptions", `{"endpoint":"http://push.example/sub"}`, participantToken, uuid.NewString())
	deactivateInvalidKey := adminHTTPRequest(engine, http.MethodDelete, "/v2/push/subscriptions", deactivateBody, participantToken, "not-a-uuid")
	deactivateForbidden := adminHTTPRequest(engine, http.MethodDelete, "/v2/push/subscriptions", deactivateBody, adminToken, uuid.NewString())
	deactivateKey := uuid.NewString()
	deactivated := adminHTTPRequest(engine, http.MethodDelete, "/v2/push/subscriptions", deactivateBody, participantToken, deactivateKey)
	deactivateRetry := adminHTTPRequest(engine, http.MethodDelete, "/v2/push/subscriptions", deactivateBody, participantToken, deactivateKey)
	deactivateKeyReused := adminHTTPRequest(engine, http.MethodDelete, "/v2/push/subscriptions", `{"endpoint":"https://push.example/other"}`, participantToken, deactivateKey)

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
		`{"title":"Aviso geral","body":"Manutenção agendada","urgent":true}`, adminToken, uuid.NewString(),
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
	require.Len(t, listBody.Data, 2)
	assert.Equal(t, messages.Uint64StringFromUint64(2), listBody.UnreadCount)
	var notificationID string
	for _, item := range listBody.Data {
		if item.Category == "announcement" {
			notificationID = item.ID
		}
	}
	require.NotEmpty(t, notificationID)

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

	// Validate the durable side effects, not only the HTTP projections.
	var pushRow models.PushSubscription
	require.NoError(t, TestSuite.DbConn.Where("endpoint = ?", pushEndpoint).Take(&pushRow).Error)
	var queueDelivery models.NotificationDelivery
	require.NoError(t, TestSuite.DbConn.Where("notification_id = ?", queueCreatedBody.NotificationID).Take(&queueDelivery).Error)
	var urgentAnnouncement models.Notification
	require.NoError(t, TestSuite.DbConn.Where("user_id = ? AND category = ?", participant.ID, "announcement").Take(&urgentAnnouncement).Error)
	var operationCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.IdempotencyOperation{}).Where("actor_user_id = ?", participant.ID).Count(&operationCount).Error)

	assert.Equal(t, http.StatusServiceUnavailable, pushUnavailable.Code)
	assert.Equal(t, http.StatusOK, pushConfig.Code, pushConfig.Body.String())
	assert.Contains(t, pushConfig.Body.String(), "test-vapid-public-key")
	assert.Equal(t, "private, no-store", pushConfig.Header().Get("Cache-Control"))
	assert.Equal(t, http.StatusForbidden, adminPushConfig.Code)
	for name, response := range map[string]int{
		"pushUnknownField": pushUnknownField.Code, "pushInvalidEndpoint": pushInvalidEndpoint.Code, "pushInvalidKey": pushInvalidKey.Code,
		"deactivateUnknownField": deactivateUnknownField.Code, "deactivateInvalidEndpoint": deactivateInvalidEndpoint.Code, "deactivateInvalidKey": deactivateInvalidKey.Code,
		"queueUnknownField": queueUnknownField.Code, "queueInvalidKey": queueInvalidKey.Code, "queueInvalidRequest": queueInvalidRequest.Code,
	} {
		assert.Equal(t, http.StatusBadRequest, response, name)
	}
	assert.Equal(t, http.StatusForbidden, pushForbidden.Code)
	assert.Equal(t, http.StatusOK, pushRetry.Code, pushRetry.Body.String())
	assert.Contains(t, pushRetry.Body.String(), pushCreatedBody.ID)
	assert.Equal(t, http.StatusConflict, pushKeyReused.Code)
	assert.Equal(t, http.StatusOK, pushRotated.Code, pushRotated.Body.String())
	assert.Equal(t, http.StatusUnauthorized, queueMissingCredential.Code)
	assert.Equal(t, http.StatusUnauthorized, queueWrongCredential.Code)
	assert.Equal(t, http.StatusOK, queueRetry.Code, queueRetry.Body.String())
	assert.Contains(t, queueRetry.Body.String(), queueCreatedBody.NotificationID)
	assert.Equal(t, http.StatusForbidden, deactivateForbidden.Code)
	assert.Equal(t, http.StatusNoContent, deactivated.Code)
	assert.Equal(t, http.StatusNoContent, deactivateRetry.Code)
	assert.Equal(t, http.StatusConflict, deactivateKeyReused.Code)
	assert.Equal(t, "inactive", pushRow.State)
	assert.Equal(t, "key-2", pushRow.P256DH)
	assert.Equal(t, pushCreatedBody.ID, pushRow.ID)
	assert.Equal(t, pushRow.ID, queueDelivery.SubscriptionID)
	assert.JSONEq(t, `{"urgent":true}`, string(urgentAnnouncement.Metadata))
	assert.Equal(t, int64(5), operationCount)
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
