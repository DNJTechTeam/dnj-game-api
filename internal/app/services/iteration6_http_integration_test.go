package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestIteration6HTTP_MiddlewareHandlerServiceRepositoryAndDatabase(t *testing.T) {
	service := setupIteration6Test(t)
	manager, _ := seedIteration6User(t, "HTTP Manager", userEntities.RoleEventManager, true, 0)
	participant, _ := seedIteration6User(t, "HTTP Participant", userEntities.RoleDefault, true, 0)
	gameID := seedIteration6Game(t, "HTTP Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)
	jwt := NewJwtService(TestSuite.BaseService)
	managerToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, manager)
	require.NoError(t, err)
	participantToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, participant)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{GameHandler: &handlers.GameHandler{GameService: service}})
	router.RegisterGameRoutes()

	catalog := adminHTTPRequest(engine, http.MethodGet, "/v2/games?page=1", "", "", "")
	detail := adminHTTPRequest(engine, http.MethodGet, "/v2/games/"+gameID, "", "", "")
	individual := adminHTTPRequest(engine, http.MethodGet, "/v2/rankings?scope=individual&page=1", "", "", "")
	groups := adminHTTPRequest(engine, http.MethodGet, "/v2/rankings?scope=groups&page=1", "", "", "")
	unauthenticated := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/game-overview", "", "", "")
	managerOverview := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/game-overview", "", managerToken, "")

	// Create and update a manager-owned game through the complete HTTP stack,
	// then use that persisted game in the run lifecycle below.
	createdGame := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/games", `{"name":"  Jogo criado por HTTP  "}`, managerToken, uuid.NewString())
	require.Equal(t, http.StatusCreated, createdGame.Code, createdGame.Body.String())
	var managerGame messages.ManagerGameResponseDTO
	require.NoError(t, json.Unmarshal(createdGame.Body.Bytes(), &managerGame))
	updatedGame := adminHTTPRequest(engine, http.MethodPatch, "/v2/manager/games/"+managerGame.ID, `{"name":"Jogo atualizado por HTTP"}`, managerToken, uuid.NewString())
	createUnknownQuery := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/games?unexpected=1", `{"name":"Inválido"}`, managerToken, uuid.NewString())
	createUnknownField := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/games", `{"name":"Inválido","actorId":"1"}`, managerToken, uuid.NewString())
	createInvalidName := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/games", `{"name":" "}`, managerToken, uuid.NewString())
	updateUnknownQuery := adminHTTPRequest(engine, http.MethodPatch, "/v2/manager/games/"+managerGame.ID+"?unexpected=1", `{"name":"Inválido"}`, managerToken, uuid.NewString())
	updateUnknownField := adminHTTPRequest(engine, http.MethodPatch, "/v2/manager/games/"+managerGame.ID, `{"name":"Inválido","actorId":"1"}`, managerToken, uuid.NewString())
	updateMissingGame := adminHTTPRequest(engine, http.MethodPatch, "/v2/manager/games/"+uuid.NewString(), `{"name":"Não encontrado"}`, managerToken, uuid.NewString())

	created := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/runs", `{"gameId":"`+managerGame.ID+`"}`, managerToken, uuid.NewString())
	var run messages.ManagerRunResponseDTO
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &run))
	managerRun := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/runs/"+run.ID, "", managerToken, "")
	qrResponse := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/qr", "", managerToken, uuid.NewString())
	var qr messages.QRResponseDTO
	require.NoError(t, json.Unmarshal(qrResponse.Body.Bytes(), &qr))
	joined := adminHTTPRequest(engine, http.MethodPost, "/v2/qr/validate", `{"qrToken":"`+qr.QRToken+`"}`, participantToken, uuid.NewString())
	currentRun := adminHTTPRequest(engine, http.MethodGet, "/v2/activity-runs/current", "", participantToken, "")
	currentParticipation := adminHTTPRequest(engine, http.MethodGet, "/v2/participations/current", "", participantToken, "")
	gameOverview := adminHTTPRequest(engine, http.MethodGet, "/v2/game/overview", "", participantToken, "")
	managerRunAfterJoin := adminHTTPRequest(engine, http.MethodGet, "/v2/manager/runs/"+run.ID, "", managerToken, "")
	var runAfterJoin messages.ManagerRunResponseDTO
	require.NoError(t, json.Unmarshal(managerRunAfterJoin.Body.Bytes(), &runAfterJoin))
	require.Len(t, runAfterJoin.Participants, 1)

	started := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/start", "", managerToken, uuid.NewString())
	paused := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/pause", "", managerToken, uuid.NewString())
	resumed := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/resume", "", managerToken, uuid.NewString())
	resultsBody := `{"results":[{"participantId":"` + runAfterJoin.Participants[0].ID + `","result":"first"}]}`
	completed := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/results", resultsBody, managerToken, uuid.NewString())
	terminalCurrent := adminHTTPRequest(engine, http.MethodGet, "/v2/activity-runs/current?runId="+run.ID, "", participantToken, "")
	noCurrent := adminHTTPRequest(engine, http.MethodGet, "/v2/activity-runs/current", "", participantToken, "")
	noParticipation := adminHTTPRequest(engine, http.MethodGet, "/v2/participations/current", "", participantToken, "")

	secondCreated := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/runs", `{"gameId":"`+gameID+`"}`, managerToken, uuid.NewString())
	var secondRun messages.ManagerRunResponseDTO
	require.NoError(t, json.Unmarshal(secondCreated.Body.Bytes(), &secondRun))
	cancelled := adminHTTPRequest(engine, http.MethodPost, "/v2/manager/runs/"+secondRun.ID+"/cancel", "", managerToken, uuid.NewString())

	for name, response := range map[string]int{
		"catalog": catalog.Code, "detail": detail.Code, "individual": individual.Code,
		"groups": groups.Code, "managerOverview": managerOverview.Code, "managerRun": managerRun.Code,
		"updatedGame": updatedGame.Code,
		"currentRun":  currentRun.Code, "currentParticipation": currentParticipation.Code,
		"gameOverview": gameOverview.Code, "managerRunAfterJoin": managerRunAfterJoin.Code,
		"started": started.Code, "paused": paused.Code, "resumed": resumed.Code,
		"completed": completed.Code, "terminalCurrent": terminalCurrent.Code, "cancelled": cancelled.Code,
	} {
		assert.Equal(t, http.StatusOK, response, name)
	}
	for name, response := range map[string]int{"createdGame": createdGame.Code, "created": created.Code, "qr": qrResponse.Code, "joined": joined.Code, "secondCreated": secondCreated.Code} {
		assert.Equal(t, http.StatusCreated, response, name)
	}
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	for name, response := range map[string]*httptest.ResponseRecorder{
		"createUnknownQuery": createUnknownQuery, "createUnknownField": createUnknownField, "createInvalidName": createInvalidName,
		"updateUnknownQuery": updateUnknownQuery, "updateUnknownField": updateUnknownField,
	} {
		assert.Equal(t, http.StatusBadRequest, response.Code, "%s: %s", name, response.Body.String())
		assert.Contains(t, response.Body.String(), "INVALID_REQUEST", name)
	}
	assert.Equal(t, http.StatusNotFound, updateMissingGame.Code, updateMissingGame.Body.String())
	assert.Contains(t, updateMissingGame.Body.String(), "NOT_FOUND")
	assert.Equal(t, http.StatusNoContent, noCurrent.Code)
	assert.Equal(t, http.StatusNoContent, noParticipation.Code)
	assert.Contains(t, catalog.Body.String(), `"name":"HTTP Game"`)
	assert.Contains(t, individual.Body.String(), `"generatedAt":"2026-10-18T15:00:00.123Z"`)
	assert.Contains(t, managerOverview.Body.String(), `"scope":"actions"`)
	assert.Contains(t, createdGame.Body.String(), `"name":"Jogo criado por HTTP"`)
	assert.Contains(t, updatedGame.Body.String(), `"name":"Jogo atualizado por HTTP"`)
	assert.NotContains(t, joined.Body.String(), "qrToken")
	assert.Contains(t, completed.Body.String(), `"status":"completed"`)
	assert.Contains(t, cancelled.Body.String(), `"status":"cancelled"`)

	var persisted models.Activity
	require.NoError(t, TestSuite.DbConn.Where("id = ?", managerGame.ID).Take(&persisted).Error)
	assert.Equal(t, "Jogo atualizado por HTTP", persisted.Name)
	assert.Equal(t, string(activityEntities.KindCompetitive), persisted.Kind)
	var gameAudits int64
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).
		Where("entity_id = ? AND action IN ?", managerGame.ID, []string{"manager.game.create", "manager.game.update"}).
		Count(&gameAudits).Error)
	assert.Equal(t, int64(2), gameAudits)
}
