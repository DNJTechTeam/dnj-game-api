package services

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
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

func TestIteration6_AdminCheckpointQR(t *testing.T) {
	// given — a checkpoint created before any QR exists, and two independent admins.
	service := setupIteration6Test(t)
	admin, adminCtx := seedIteration6User(t, "Admin", userEntities.RoleAdmin, true, 0)
	_, otherAdminCtx := seedIteration6User(t, "Other admin", userEntities.RoleAdmin, true, 0)
	_, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	_, participantCtx := seedIteration6User(t, "Participant", userEntities.RoleDefault, true, 0)
	activityID := seedIteration6Game(t, "Checkpoint", activityEntities.StatusActive, nil)
	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("id = ?", activityID).Updates(map[string]any{"kind": "checkpoint", "check_in_points": 15}).Error)
	token, err := NewJwtService(TestSuite.BaseService).GenerateIdentityToken(TestSuite.Ctx, admin)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	routers.NewRouter(engine, &handlers.Handlers{GameHandler: &handlers.GameHandler{GameService: service}}).RegisterGameRoutes()
	path := "/v2/admin/activities/" + activityID + "/qr"
	assert.Equal(t, http.StatusNoContent, adminHTTPRequest(engine, http.MethodGet, path, "", token, "").Code)
	assert.Equal(t, http.StatusUnauthorized, adminHTTPRequest(engine, http.MethodGet, path, "", "", "").Code)
	assert.Equal(t, http.StatusBadRequest, adminHTTPRequest(engine, http.MethodGet, path+"?unexpected=1", "", token, "").Code)

	// when — reading does not create anything, even if a run exists without a QR.
	empty, err := service.AdminCheckpointQR(adminCtx, activityID)
	require.NoError(t, err)
	assert.Nil(t, empty)
	run := createIteration6Run(t, service, adminCtx, activityID)
	empty, err = service.AdminCheckpointQR(adminCtx, activityID)
	require.NoError(t, err)
	assert.Nil(t, empty)
	original := rotateIteration6QR(t, service, adminCtx, run.ID)
	httpQR := adminHTTPRequest(engine, http.MethodGet, path, "", token, "")
	require.Equal(t, http.StatusOK, httpQR.Code, httpQR.Body.String())
	var saved messages.QRResponseDTO
	require.NoError(t, json.Unmarshal(httpQR.Body.Bytes(), &saved))
	assert.Equal(t, original.QRToken, saved.QRToken)
	assert.Equal(t, http.StatusNotFound, adminHTTPRequest(engine, http.MethodGet, "/v2/admin/activities/"+uuid.NewString()+"/qr", "", token, "").Code)

	// then — every admin recovers the persisted QR, also through repeated creation requests.
	recovered, err := service.AdminCheckpointQR(otherAdminCtx, activityID)
	require.NoError(t, err)
	assert.Equal(t, original.QRID, recovered.QRID)
	assert.Equal(t, original.QRToken, recovered.QRToken)
	assert.WithinDuration(t, original.ExpiresAt, recovered.ExpiresAt, time.Microsecond)
	reusedRun, status, err := service.CreateRun(otherAdminCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: activityID})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, run.ID, reusedRun.ID)
	reusedQR, status, err := service.RotateQR(otherAdminCtx, run.ID, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, original.QRToken, reusedQR.QRToken)
	assert.Equal(t, 15, validateIteration6QR(t, service, participantCtx, original.QRToken).PointsAwarded)

	// given — pausing the activity must not hide or replace its printable code.
	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("id = ?", activityID).Update("status", "paused").Error)
	recovered, err = service.AdminCheckpointQR(adminCtx, activityID)
	require.NoError(t, err)
	assert.Equal(t, original.QRToken, recovered.QRToken)
	var runs, codes int64
	require.NoError(t, TestSuite.DbConn.Model(&models.ActivityRun{}).Where("activity_id = ?", activityID).Count(&runs).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.ActivityRunQRCode{}).Where("activity_id = ?", activityID).Count(&codes).Error)
	assert.EqualValues(t, 1, runs)
	assert.EqualValues(t, 1, codes)

	// then — this admin read remains private and rejects other activity kinds.
	_, err = service.AdminCheckpointQR(managerCtx, activityID)
	apiServiceError(t, err, http.StatusForbidden, "FORBIDDEN")
	_, err = service.AdminCheckpointQR(participantCtx, activityID)
	apiServiceError(t, err, http.StatusForbidden, "FORBIDDEN")
	_, err = service.AdminCheckpointQR(adminCtx, uuid.NewString())
	apiServiceError(t, err, http.StatusNotFound, "NOT_FOUND")
	_, err = service.AdminCheckpointQR(adminCtx, "invalid")
	apiServiceError(t, err, http.StatusNotFound, "NOT_FOUND")
	gameID := seedIteration6Game(t, "Game", activityEntities.StatusActive, nil)
	_, err = service.AdminCheckpointQR(adminCtx, gameID)
	apiServiceError(t, err, http.StatusNotFound, "NOT_FOUND")
}

func TestIteration6_ConcurrentCheckpointQR(t *testing.T) {
	// given — two admins generating the first QR at the same time.
	service := setupIteration6Test(t)
	_, firstCtx := seedIteration6User(t, "First", userEntities.RoleAdmin, true, 0)
	_, secondCtx := seedIteration6User(t, "Second", userEntities.RoleAdmin, true, 0)
	activityID := seedIteration6Game(t, "Checkpoint", activityEntities.StatusActive, nil)
	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("id = ?", activityID).Update("kind", "checkpoint").Error)
	var wg sync.WaitGroup
	results := make([]*messages.QRResponseDTO, 2)
	errors := make([]error, 2)

	// when
	for i, ctx := range []context.Context{firstCtx, secondCtx} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run, _, err := service.CreateRun(ctx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: activityID})
			if err != nil {
				errors[i] = err
				return
			}
			results[i], _, errors[i] = service.RotateQR(ctx, run.ID, uuid.NewString())
		}()
	}
	wg.Wait()

	// then — both requests succeed with one persisted code and run.
	require.NoError(t, errors[0])
	require.NoError(t, errors[1])
	assert.Equal(t, results[0].RunID, results[1].RunID)
	assert.Equal(t, results[0].QRToken, results[1].QRToken)
}
