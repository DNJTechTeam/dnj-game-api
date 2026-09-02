package services

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/repositories"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var specialEventHTTPNow = time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)

func setupSpecialEventHTTPTest(t *testing.T) (*SpecialEventService, *GameService) {
	t.Helper()
	TestSuite.DefaultSetup(t)
	for _, model := range []interface{ TableName() string }{
		&models.NotificationDelivery{}, &models.PushSubscription{}, &models.Notification{}, &models.NotificationPreference{},
		&models.SpecialEvent{}, &models.PointEntry{}, &models.ActivityRunParticipant{}, &models.Participation{},
		&models.ActivityRunQRCode{}, &models.ActivityRun{}, &models.ParticipantOperation{}, &models.ManagerOperation{},
		&models.ActivityManagerAssignment{}, &models.OperationAudit{}, &models.Activity{}, &models.User{},
	} {
		TestSuite.TruncateTable(t, model)
	}

	notifications := repositories.NewNotificationRepository(TestSuite.DbConn)
	events := repositories.NewSpecialEventRepository(TestSuite.DbConn)
	special := NewSpecialEventService(TestSuite.BaseService, events, TestSuite.ActivityRepository, TestSuite.GameRepository, TestSuite.UserRepository, notifications).(*SpecialEventService)
	require.Equal(t, "test-document-hmac-secret", special.secret())
	special.now = func() time.Time { return specialEventHTTPNow }
	special.secret = func() string { return "special-event-http-secret" }
	game := NewGameService(TestSuite.BaseService, TestSuite.GameRepository, TestSuite.ActivityRepository, TestSuite.UserRepository, TestSuite.OperationAuditRepository).(*GameService)
	game.now = func() time.Time { return specialEventHTTPNow }
	game.secret = func() string { return "special-event-http-secret" }
	return special, game
}

func TestIteration6_SpecialEventsHTTPFullLifecycleAwardsPointsAndNotifies(t *testing.T) {
	// given
	special, game := setupSpecialEventHTTPTest(t)
	manager, _ := seedIteration6User(t, "Special Event Manager", userEntities.RoleEventManager, true, 0)
	actionsManager, _ := seedIteration6User(t, "Actions Manager", userEntities.RoleEventManager, true, 0)
	participant, _ := seedIteration6User(t, "Special Event Participant", userEntities.RoleDefault, true, 0)
	admin, _ := seedIteration6User(t, "Special Event Admin", userEntities.RoleAdmin, true, 0)
	removedManager, removedManagerCtx := seedIteration6User(t, "Removed Special Manager", userEntities.RoleEventManager, true, 0)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", manager.ID).Update("manager_scope", "special_events").Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", actionsManager.ID).Update("manager_scope", "actions").Error)

	jwt := NewJwtService(TestSuite.BaseService)
	managerToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, manager)
	require.NoError(t, err)
	actionsToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, actionsManager)
	require.NoError(t, err)
	participantToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, participant)
	require.NoError(t, err)
	adminToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, admin)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{
		GameHandler:         &handlers.GameHandler{GameService: game},
		SpecialEventHandler: &handlers.SpecialEventHandler{Service: special},
	})
	router.RegisterGameRoutes()
	router.RegisterSpecialEventRoutes()

	// when: authentication, role/scope enforcement, and strict validation.
	unauthenticated := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/special-events", "", "", "")
	participantCreate := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"Proibido","points":10,"durationMinutes":30,"targets":["app"]}`, participantToken, "")
	wrongScopeCreate := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"Proibido","points":10,"durationMinutes":30,"targets":["app"]}`, actionsToken, "")
	invalidCreate := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"Inválido","points":-1,"durationMinutes":0,"targets":["app","app"]}`, managerToken, "")
	invalidTargets := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"Inválido","points":10,"durationMinutes":30,"targets":["app","app"]}`, managerToken, "")
	emptyTargets := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"Inválido","points":10,"durationMinutes":30,"targets":[]}`, managerToken, "")
	unknownTarget := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"Inválido","points":10,"durationMinutes":30,"targets":["web"]}`, managerToken, "")
	unknownCreate := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"Inválido","points":10,"durationMinutes":30,"targets":["app"],"actorId":"1"}`, managerToken, "")
	_, unauthenticatedServiceErr := special.ListManager(TestSuite.Ctx)
	require.NoError(t, TestSuite.DbConn.Delete(&models.User{}, removedManager.ID).Error)
	_, removedManagerErr := special.ListManager(removedManagerCtx)

	// when: create, publish the teaser, release the QR, and consume it through
	// the game endpoint. Every operation uses the real repositories/database.
	createdResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"  Caça ao tesouro  ","description":"  Encontre as pistas  ","points":25,"durationMinutes":30,"targets":["app","tv","screen"]}`, managerToken, "")
	require.Equal(t, http.StatusCreated, createdResponse.Code, createdResponse.Body.String())
	var created messages.ManagerSpecialEventDTO
	require.NoError(t, json.Unmarshal(createdResponse.Body.Bytes(), &created))

	listResponse := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/special-events", "", managerToken, "")
	adminList := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/special-events", "", adminToken, "")
	wrongScopeList := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/special-events", "", actionsToken, "")
	listUnknownQuery := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/special-events?unexpected=1", "", managerToken, "")
	draftToCloseResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events", `{"title":"Evento sem QR","description":" ","points":5,"durationMinutes":10,"targets":["app"]}`, managerToken, "")
	require.Equal(t, http.StatusCreated, draftToCloseResponse.Code, draftToCloseResponse.Body.String())
	var draftToClose messages.ManagerSpecialEventDTO
	require.NoError(t, json.Unmarshal(draftToCloseResponse.Body.Bytes(), &draftToClose))
	draftQR := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/qr", `{"eventId":"`+draftToClose.ID+`"}`, managerToken, "")
	draftClose := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/close", `{"eventId":"`+draftToClose.ID+`"}`, managerToken, "")
	draftActive := adminHTTPRequest(engine, http.MethodGet, "/v2/special-events/active?target=app", "", participantToken, "")
	teaserUnknownField := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/teaser", `{"eventId":"`+created.ID+`","actorId":"1"}`, managerToken, "")
	teaserMissing := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/teaser", `{"eventId":"`+uuid.NewString()+`"}`, managerToken, "")
	participantTeaser := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/teaser", `{"eventId":"`+created.ID+`"}`, participantToken, "")
	teaserResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/teaser", `{"eventId":"`+created.ID+`"}`, managerToken, "")
	repeatedTeaser := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/teaser", `{"eventId":"`+created.ID+`"}`, managerToken, "")
	teaserActive := adminHTTPRequest(engine, http.MethodGet, "/v2/special-events/active?target=app", "", participantToken, "")
	teaserDisplay := adminHTTPRequest(engine, http.MethodGet, "/v2/live-display?target=tv", "", "", "")
	var teaserActiveBody messages.ActiveSpecialEventResponseDTO
	require.NoError(t, json.Unmarshal(teaserActive.Body.Bytes(), &teaserActiveBody))
	var teaserDisplayBody messages.LiveDisplaySpecialEventDTO
	require.NoError(t, json.Unmarshal(teaserDisplay.Body.Bytes(), &teaserDisplayBody))
	special.secret = func() string { return "" }
	missingSecretQR := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/qr", `{"eventId":"`+created.ID+`"}`, managerToken, "")
	special.secret = func() string { return "special-event-http-secret" }
	earlyQR := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/qr", `{"eventId":"`+created.ID+`"}`, managerToken, "")

	special.now = func() time.Time { return specialEventHTTPNow.Add(16 * time.Second) }
	game.now = func() time.Time { return specialEventHTTPNow.Add(16 * time.Second) }
	qrUnknownField := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/qr", `{"eventId":"`+created.ID+`","actorId":"1"}`, managerToken, "")
	qrMissing := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/qr", `{"eventId":"`+uuid.NewString()+`"}`, managerToken, "")
	participantQR := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/qr", `{"eventId":"`+created.ID+`"}`, participantToken, "")
	qrResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/qr", `{"eventId":"`+created.ID+`"}`, managerToken, "")
	require.Equal(t, http.StatusOK, qrResponse.Code, qrResponse.Body.String())
	var qr messages.SpecialEventQRResponseDTO
	require.NoError(t, json.Unmarshal(qrResponse.Body.Bytes(), &qr))
	rotatedQRResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/qr", `{"eventId":"`+created.ID+`"}`, managerToken, "")
	require.Equal(t, http.StatusOK, rotatedQRResponse.Code, rotatedQRResponse.Body.String())
	var rotatedQR messages.SpecialEventQRResponseDTO
	require.NoError(t, json.Unmarshal(rotatedQRResponse.Body.Bytes(), &rotatedQR))
	activeResponse := adminHTTPRequest(engine, http.MethodGet, "/v2/special-events/active?target=app", "", participantToken, "")
	displayResponse := adminHTTPRequest(engine, http.MethodGet, "/v2/live-display?target=screen", "", "", "")
	rotatedOutResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/qr/validate", `{"qrToken":"`+qr.QRToken+`"}`, participantToken, uuid.NewString())
	scoredResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/qr/validate", `{"qrToken":"`+rotatedQR.QRToken+`"}`, participantToken, uuid.NewString())
	closeUnknownField := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/close", `{"eventId":"`+created.ID+`","actorId":"1"}`, managerToken, "")
	closeMissing := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/close", `{"eventId":"`+uuid.NewString()+`"}`, managerToken, "")
	participantClose := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/close", `{"eventId":"`+created.ID+`"}`, participantToken, "")
	closeResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/special-events/close", `{"eventId":"`+created.ID+`"}`, managerToken, "")
	closedActive := adminHTTPRequest(engine, http.MethodGet, "/v2/special-events/active?target=app", "", participantToken, "")
	closedDisplay := adminHTTPRequest(engine, http.MethodGet, "/v2/live-display?target=tv", "", "", "")
	invalidActive := adminHTTPRequest(engine, http.MethodGet, "/v2/special-events/active?target=tv", "", participantToken, "")
	invalidDisplay := adminHTTPRequest(engine, http.MethodGet, "/v2/live-display?target=app", "", "", "")

	var persistedUser models.User
	require.NoError(t, TestSuite.DbConn.Where("id = ?", participant.ID).Take(&persistedUser).Error)
	var notification models.Notification
	require.NoError(t, TestSuite.DbConn.Where("user_id = ? AND category = ?", participant.ID, "special_event").Take(&notification).Error)
	var notificationCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).Where("user_id = ? AND category = ?", participant.ID, "special_event").Count(&notificationCount).Error)
	var run models.ActivityRun
	require.NoError(t, TestSuite.DbConn.Where("activity_id = ?", created.ID).Take(&run).Error)

	// then
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	assert.Equal(t, http.StatusForbidden, participantCreate.Code)
	assert.Equal(t, http.StatusForbidden, wrongScopeCreate.Code)
	assert.Equal(t, http.StatusBadRequest, invalidCreate.Code)
	assert.Equal(t, http.StatusBadRequest, invalidTargets.Code)
	assert.Equal(t, http.StatusBadRequest, emptyTargets.Code)
	assert.Equal(t, http.StatusBadRequest, unknownTarget.Code)
	assert.Equal(t, http.StatusBadRequest, unknownCreate.Code)
	apiServiceError(t, unauthenticatedServiceErr, http.StatusUnauthorized, "UNAUTHENTICATED")
	apiServiceError(t, removedManagerErr, http.StatusUnauthorized, "UNAUTHENTICATED")
	assert.Equal(t, "Caça ao tesouro", created.Title)
	assert.Equal(t, 25, created.Points)
	assert.Equal(t, http.StatusOK, listResponse.Code, listResponse.Body.String())
	assert.Contains(t, listResponse.Body.String(), `"title":"Caça ao tesouro"`)
	assert.Equal(t, http.StatusOK, adminList.Code, adminList.Body.String())
	assert.Equal(t, http.StatusForbidden, wrongScopeList.Code)
	assert.Equal(t, http.StatusBadRequest, listUnknownQuery.Code)
	assert.Equal(t, http.StatusConflict, draftQR.Code)
	assert.Contains(t, draftQR.Body.String(), "EVENT_STATE_CONFLICT")
	assert.Equal(t, http.StatusOK, draftClose.Code, draftClose.Body.String())
	assert.Equal(t, http.StatusOK, draftActive.Code)
	assert.Contains(t, draftActive.Body.String(), `"event":null`)
	assert.Equal(t, http.StatusBadRequest, teaserUnknownField.Code)
	assert.Equal(t, http.StatusNotFound, teaserMissing.Code)
	assert.Equal(t, http.StatusForbidden, participantTeaser.Code)
	assert.Equal(t, http.StatusOK, teaserResponse.Code, teaserResponse.Body.String())
	assert.Contains(t, teaserResponse.Body.String(), `"status":"teaser"`)
	assert.Equal(t, http.StatusConflict, repeatedTeaser.Code)
	assert.Contains(t, repeatedTeaser.Body.String(), "EVENT_STATE_CONFLICT")
	require.NotNil(t, teaserActiveBody.Event)
	require.NotNil(t, teaserActiveBody.Event.QRAvailableAt)
	require.NotNil(t, teaserDisplayBody.ReadyAt)
	assert.Equal(t, specialEventHTTPNow.Add(15*time.Second), teaserActiveBody.Event.QRAvailableAt.UTC())
	assert.Equal(t, specialEventHTTPNow.Add(15*time.Second), teaserDisplayBody.ReadyAt.UTC())
	assert.Equal(t, http.StatusInternalServerError, missingSecretQR.Code)
	assert.Contains(t, missingSecretQR.Body.String(), "INTERNAL_ERROR")
	assert.Equal(t, http.StatusConflict, earlyQR.Code)
	assert.Contains(t, earlyQR.Body.String(), "TEASER_IN_PROGRESS")
	assert.Equal(t, http.StatusBadRequest, qrUnknownField.Code)
	assert.Equal(t, http.StatusNotFound, qrMissing.Code)
	assert.Equal(t, http.StatusForbidden, participantQR.Code)
	assert.NotEqual(t, qr.QRToken, rotatedQR.QRToken)
	assert.Contains(t, activeResponse.Body.String(), rotatedQR.QRToken)
	assert.Contains(t, displayResponse.Body.String(), rotatedQR.QRToken)
	assert.Equal(t, http.StatusConflict, rotatedOutResponse.Code)
	assert.Contains(t, rotatedOutResponse.Body.String(), "QR_UNAVAILABLE")
	assert.Equal(t, http.StatusCreated, scoredResponse.Code, scoredResponse.Body.String())
	assert.Contains(t, scoredResponse.Body.String(), `"action":"scored"`)
	assert.Contains(t, scoredResponse.Body.String(), `"pointsAwarded":25`)
	assert.Equal(t, 25, persistedUser.Points)
	assert.Equal(t, "Encontre as pistas", notification.Body)
	assert.Equal(t, int64(1), notificationCount)
	assert.Equal(t, http.StatusBadRequest, closeUnknownField.Code)
	assert.Equal(t, http.StatusNotFound, closeMissing.Code)
	assert.Equal(t, http.StatusForbidden, participantClose.Code)
	assert.Equal(t, http.StatusOK, closeResponse.Code, closeResponse.Body.String())
	assert.Equal(t, string(gameEntities.RunStatusCancelled), run.Status)
	assert.Contains(t, closedActive.Body.String(), `"event":null`)
	assert.Equal(t, http.StatusOK, closedDisplay.Code)
	assert.Equal(t, "null", closedDisplay.Body.String())
	assert.Equal(t, http.StatusBadRequest, invalidActive.Code)
	assert.Equal(t, http.StatusBadRequest, invalidDisplay.Code)
}
