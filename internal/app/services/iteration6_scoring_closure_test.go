package services

import (
	"errors"
	"net/http"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIteration6TestWithClosedScoring(t *testing.T, closed bool) *GameService {
	t.Helper()
	TestSuite.DefaultSetup(t)
	for _, model := range []interface{ TableName() string }{
		&models.ManagerOperation{}, &models.PointEntry{}, &models.ScheduleQRCheckIn{}, &models.QRScanBlock{}, &models.ActivityRunParticipant{}, &models.Participation{}, &models.ActivityRunQRCode{}, &models.ActivityRun{},
		&models.ParticipantOperation{}, &models.UserFavorite{}, &models.OperationAudit{}, &models.ActivityManagerAssignment{}, &models.GroupMembership{}, &models.Activity{}, &models.Space{}, &models.User{}, &models.Group{},
	} {
		TestSuite.TruncateTable(t, model)
	}
	fake := newFakeEventSettingsRepository()
	fake.closed = closed
	service := NewGameService(TestSuite.BaseService, TestSuite.GameRepository, TestSuite.ActivityRepository, TestSuite.UserRepository, TestSuite.OperationAuditRepository, fake).(*GameService)
	service.now = func() time.Time { return iteration6Now }
	service.secret = func() string { return "iteration-6-qr-secret" }
	return service
}

func TestIteration6_ValidateQR_ScoringClosed(t *testing.T) {
	service := setupIteration6TestWithClosedScoring(t, true)
	manager, managerCtx := seedIteration6User(t, "manager", userEntities.RoleEventManager, true, 0)
	_, participantCtx := seedIteration6User(t, "participant", userEntities.RoleDefault, true, 0)

	// Create a game and run with an open QR
	gameID := seedIteration6Game(t, "Test Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)
	run := createIteration6Run(t, service, managerCtx, gameID)
	qr := rotateIteration6QR(t, service, managerCtx, run.ID)

	// Try to validate the QR - should fail with SCORING_CLOSED
	response, status, err := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: qr.QRToken, IdempotencyKey: uuid.NewString()})

	require.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, 0, status) // status is 0 when error is returned
	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
	assert.Equal(t, "SCORING_CLOSED", apiErr.Code)
}

func TestIteration6_ValidateScheduleQR_ScoringClosed(t *testing.T) {
	service := setupIteration6TestWithClosedScoring(t, true)
	_, participantCtx := seedIteration6User(t, "participant", userEntities.RoleDefault, true, 0)

	// Create a space
	spaceID := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Space{ID: spaceID, Slug: "test-space-" + uuid.NewString(), Name: "Test Space", CreatedAt: iteration6Now, UpdatedAt: iteration6Now}).Error)

	// Create a schedule QR for the space
	scheduleQR := service.scheduleQRToken(spaceID)

	// Try to validate the schedule QR - should fail with SCORING_CLOSED
	response, status, err := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: scheduleQR, IdempotencyKey: uuid.NewString()})

	require.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, 0, status) // status is 0 when error is returned
	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
	assert.Equal(t, "SCORING_CLOSED", apiErr.Code)
}

func TestIteration6_ValidateScheduleQR_DirectScoringClosed(t *testing.T) {
	service := setupIteration6TestWithClosedScoring(t, true)

	_, _, err := service.validateScheduleQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{
		QRToken:        "schedule.valid",
		IdempotencyKey: uuid.NewString(),
	}, uuid.NewString())

	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
	assert.Equal(t, "SCORING_CLOSED", apiErr.Code)
}

func TestIteration6_ValidateQR_ScoringOpen(t *testing.T) {
	service := setupIteration6TestWithClosedScoring(t, false)
	manager, managerCtx := seedIteration6User(t, "manager", userEntities.RoleEventManager, true, 0)
	_, participantCtx := seedIteration6User(t, "participant", userEntities.RoleDefault, true, 0)

	// Create a game and run with an open QR
	gameID := seedIteration6Game(t, "Test Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)
	run := createIteration6Run(t, service, managerCtx, gameID)
	qr := rotateIteration6QR(t, service, managerCtx, run.ID)

	// Validate the QR - should succeed
	response, status, err := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: qr.QRToken, IdempotencyKey: uuid.NewString()})

	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, http.StatusCreated, status)
}

func TestIteration6_ValidateQR_ContinuesWhenScoringStatusUnavailable(t *testing.T) {
	service := setupIteration6TestWithClosedScoring(t, false)
	service.eventSettings.(*fakeEventSettingsRepository).err = errors.New("settings unavailable")

	_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{
		QRToken:        "schedule.invalid",
		IdempotencyKey: uuid.NewString(),
	})

	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
	assert.Equal(t, "QR_UNAVAILABLE", apiErr.Code)
}
