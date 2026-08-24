package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func iteration6HandlerEngine(t *testing.T, service *mocks.MockGameServiceInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := &handlers.GameHandler{GameService: service}
	engine.GET("/v2/games", handler.List)
	engine.GET("/v2/games/:gameId", handler.Get)
	engine.GET("/v2/rankings", handler.Rankings)
	engine.GET("/v2/game/overview", handler.Overview)
	engine.GET("/v2/activity-runs/current", handler.CurrentRun)
	engine.GET("/v2/participations/current", handler.CurrentParticipation)
	engine.POST("/v2/qr/validate", handler.ValidateQR)
	engine.GET("/v2/manager/game-overview", handler.ManagerOverview)
	engine.GET("/v2/manager/runs/:runId", handler.ManagerRun)
	engine.POST("/v2/manager/runs", handler.CreateRun)
	engine.POST("/v2/manager/runs/:runId/qr", handler.RotateQR)
	engine.POST("/v2/manager/runs/:runId/start", handler.StartRun)
	engine.POST("/v2/manager/runs/:runId/pause", handler.PauseRun)
	engine.POST("/v2/manager/runs/:runId/resume", handler.ResumeRun)
	engine.POST("/v2/manager/runs/:runId/results", handler.FinalizeRun)
	engine.POST("/v2/manager/runs/:runId/cancel", handler.CancelRun)
	return engine
}

func iteration6Request(engine http.Handler, method, path, body, key string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestIteration6Handlers_AllEndpointsAndNoContent(t *testing.T) {
	// given
	service := mocks.NewMockGameServiceInterface(t)
	now := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	page := messages.Pagination{CurrentPage: 1, Limit: 10}
	run := &messages.ManagerRunResponseDTO{ID: "11111111-1111-4111-8111-111111111111", Status: "draft", Participants: []messages.RunParticipantResponseDTO{}}
	service.On("ListGames", mock.Anything, mock.MatchedBy(func(filter *messages.ListGamesFilterDTO) bool { return filter.GetPage() == 1 })).Return(&messages.PaginatedResponse[messages.GameResponseDTO]{Data: []messages.GameResponseDTO{}, Pagination: page}, nil).Once()
	service.On("GetGame", mock.Anything, "game-id").Return(&messages.GameResponseDTO{ID: "game-id", Name: "Game"}, nil).Once()
	service.On("Rankings", mock.Anything, "individual", uint64(0)).Return(&messages.RankingResponseDTO{Data: []messages.IndividualRankingResponseDTO{}, Pagination: page, GeneratedAt: now}, nil).Once()
	service.On("Overview", mock.Anything).Return(&messages.GameOverviewResponseDTO{Individual: []messages.IndividualRankingResponseDTO{}, Groups: []messages.GroupRankingResponseDTO{}, PointEntries: []messages.PointEntryResponseDTO{}}, nil).Once()
	service.On("CurrentRun", mock.Anything, "").Return(nil, nil).Once()
	service.On("CurrentParticipation", mock.Anything).Return(nil, nil).Once()
	service.On("ValidateQR", mock.Anything, mock.MatchedBy(func(request *messages.QRValidateRequestDTO) bool {
		return request.QRToken == "token" && request.IdempotencyKey != ""
	})).Return(&messages.ParticipationEnvelopeDTO{}, http.StatusCreated, nil).Once()
	service.On("ManagerOverview", mock.Anything).Return(&messages.ManagerGameOverviewResponseDTO{Scope: "actions"}, nil).Once()
	service.On("ManagerRun", mock.Anything, run.ID).Return(run, nil).Once()
	service.On("CreateRun", mock.Anything, "key", mock.MatchedBy(func(request *messages.CreateRunRequestDTO) bool { return request.GameID == "game-id" })).Return(run, http.StatusCreated, nil).Once()
	service.On("RotateQR", mock.Anything, run.ID, "key").Return(&messages.QRResponseDTO{RunID: run.ID, QRID: "qr", QRToken: "token", ExpiresAt: now}, http.StatusCreated, nil).Once()
	service.On("StartRun", mock.Anything, run.ID, "key").Return(run, nil).Once()
	service.On("PauseRun", mock.Anything, run.ID, "key").Return(run, nil).Once()
	service.On("ResumeRun", mock.Anything, run.ID, "key").Return(run, nil).Once()
	service.On("FinalizeRun", mock.Anything, run.ID, "key", mock.MatchedBy(func(request *messages.FinalizeRunResultsRequestDTO) bool { return len(request.Results) == 1 })).Return(run, nil).Once()
	service.On("CancelRun", mock.Anything, run.ID, "key").Return(run, nil).Once()
	engine := iteration6HandlerEngine(t, service)

	// when
	responses := []*httptest.ResponseRecorder{
		iteration6Request(engine, http.MethodGet, "/v2/games?page=2", "", ""),
		iteration6Request(engine, http.MethodGet, "/v2/games/game-id", "", ""),
		iteration6Request(engine, http.MethodGet, "/v2/rankings?scope=individual", "", ""),
		iteration6Request(engine, http.MethodGet, "/v2/game/overview", "", ""),
		iteration6Request(engine, http.MethodGet, "/v2/activity-runs/current", "", ""),
		iteration6Request(engine, http.MethodGet, "/v2/participations/current", "", ""),
		iteration6Request(engine, http.MethodPost, "/v2/qr/validate", `{"qrToken":"token"}`, "22222222-2222-4222-8222-222222222222"),
		iteration6Request(engine, http.MethodGet, "/v2/manager/game-overview", "", ""),
		iteration6Request(engine, http.MethodGet, "/v2/manager/runs/"+run.ID, "", ""),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs", `{"gameId":"game-id"}`, "key"),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/qr", "", "key"),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/start", "", "key"),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/pause", "", "key"),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/resume", "", "key"),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/results", `{"results":[{"participantId":"p","result":"first"}]}`, "key"),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+run.ID+"/cancel", "", "key"),
	}

	// then
	for index, response := range responses {
		expected := http.StatusOK
		if index == 4 || index == 5 {
			expected = http.StatusNoContent
		}
		if index == 6 || index == 9 || index == 10 {
			expected = http.StatusCreated
		}
		assert.Equal(t, expected, response.Code, "response %d: %s", index, response.Body.String())
	}
}

func TestIteration6Handlers_RejectUnknownRepeatedBodiesAndPages(t *testing.T) {
	// given
	service := mocks.NewMockGameServiceInterface(t)
	engine := iteration6HandlerEngine(t, service)
	runID := "11111111-1111-4111-8111-111111111111"

	// when
	responses := []*httptest.ResponseRecorder{
		iteration6Request(engine, http.MethodGet, "/v2/games?page=-1", "", ""),
		iteration6Request(engine, http.MethodGet, "/v2/games?page=1&page=2", "", ""),
		iteration6Request(engine, http.MethodGet, "/v2/rankings?scope=individual&admin=true", "", ""),
		iteration6Request(engine, http.MethodPost, "/v2/qr/validate", `{"qrToken":"x","idempotencyKey":"k","userId":"9"}`, ""),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs", `{"gameId":"g","points":999}`, "key"),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+runID+"/start", `{}`, "key"),
		iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+runID+"/results", `{"results":[],"actorId":"9"}`, "key"),
	}

	// then
	for _, response := range responses {
		assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "INVALID_REQUEST")
	}
}

func TestIteration6Handlers_MapsPublishedErrors(t *testing.T) {
	for _, testCase := range []struct {
		status int
		code   string
	}{
		{400, "INVALID_REQUEST"}, {401, "UNAUTHENTICATED"}, {403, "FORBIDDEN"}, {404, "NOT_FOUND"}, {409, "RUN_STATE_CONFLICT"}, {410, "QR_EXPIRED"}, {429, "RATE_LIMITED"}, {500, "INTERNAL_ERROR"},
	} {
		t.Run(testCase.code, func(t *testing.T) {
			// given
			service := mocks.NewMockGameServiceInterface(t)
			service.On("ManagerRun", mock.Anything, "run").Return(nil, appErrors.NewAPIServiceError(testCase.status, testCase.code, "failure", nil)).Once()
			engine := iteration6HandlerEngine(t, service)

			// when
			response := iteration6Request(engine, http.MethodGet, "/v2/manager/runs/run", "", "")

			// then
			assert.Equal(t, testCase.status, response.Code)
			assert.Contains(t, response.Body.String(), testCase.code)
		})
	}
}

func TestIteration6Handlers_EachEndpointMapsServiceFailures(t *testing.T) {
	failure := appErrors.NewAPIServiceError(http.StatusInternalServerError, "INTERNAL_ERROR", "failure", nil)
	runID := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		key    string
		setup  func(*mocks.MockGameServiceInterface)
	}{
		{"games", http.MethodGet, "/v2/games", "", "", func(service *mocks.MockGameServiceInterface) {
			service.On("ListGames", mock.Anything, mock.Anything).Return(nil, failure)
		}},
		{"game", http.MethodGet, "/v2/games/game", "", "", func(service *mocks.MockGameServiceInterface) {
			service.On("GetGame", mock.Anything, "game").Return(nil, failure)
		}},
		{"rankings", http.MethodGet, "/v2/rankings?scope=individual", "", "", func(service *mocks.MockGameServiceInterface) {
			service.On("Rankings", mock.Anything, "individual", uint64(0)).Return(nil, failure)
		}},
		{"overview", http.MethodGet, "/v2/game/overview", "", "", func(service *mocks.MockGameServiceInterface) {
			service.On("Overview", mock.Anything).Return(nil, failure)
		}},
		{"current-run", http.MethodGet, "/v2/activity-runs/current", "", "", func(service *mocks.MockGameServiceInterface) {
			service.On("CurrentRun", mock.Anything, "").Return(nil, failure)
		}},
		{"participation", http.MethodGet, "/v2/participations/current", "", "", func(service *mocks.MockGameServiceInterface) {
			service.On("CurrentParticipation", mock.Anything).Return(nil, failure)
		}},
		{"validate-qr", http.MethodPost, "/v2/qr/validate", `{"qrToken":"token"}`, "22222222-2222-4222-8222-222222222222", func(service *mocks.MockGameServiceInterface) {
			service.On("ValidateQR", mock.Anything, mock.Anything).Return(nil, 0, failure)
		}},
		{"manager-overview", http.MethodGet, "/v2/manager/game-overview", "", "", func(service *mocks.MockGameServiceInterface) {
			service.On("ManagerOverview", mock.Anything).Return(nil, failure)
		}},
		{"manager-run", http.MethodGet, "/v2/manager/runs/" + runID, "", "", func(service *mocks.MockGameServiceInterface) {
			service.On("ManagerRun", mock.Anything, runID).Return(nil, failure)
		}},
		{"create", http.MethodPost, "/v2/manager/runs", `{"gameId":"game"}`, "key", func(service *mocks.MockGameServiceInterface) {
			service.On("CreateRun", mock.Anything, "key", mock.Anything).Return(nil, 0, failure)
		}},
		{"qr", http.MethodPost, "/v2/manager/runs/" + runID + "/qr", "", "key", func(service *mocks.MockGameServiceInterface) {
			service.On("RotateQR", mock.Anything, runID, "key").Return(nil, 0, failure)
		}},
		{"start", http.MethodPost, "/v2/manager/runs/" + runID + "/start", "", "key", func(service *mocks.MockGameServiceInterface) {
			service.On("StartRun", mock.Anything, runID, "key").Return(nil, failure)
		}},
		{"pause", http.MethodPost, "/v2/manager/runs/" + runID + "/pause", "", "key", func(service *mocks.MockGameServiceInterface) {
			service.On("PauseRun", mock.Anything, runID, "key").Return(nil, failure)
		}},
		{"resume", http.MethodPost, "/v2/manager/runs/" + runID + "/resume", "", "key", func(service *mocks.MockGameServiceInterface) {
			service.On("ResumeRun", mock.Anything, runID, "key").Return(nil, failure)
		}},
		{"results", http.MethodPost, "/v2/manager/runs/" + runID + "/results", `{"results":[]}`, "key", func(service *mocks.MockGameServiceInterface) {
			service.On("FinalizeRun", mock.Anything, runID, "key", mock.Anything).Return(nil, failure)
		}},
		{"cancel", http.MethodPost, "/v2/manager/runs/" + runID + "/cancel", "", "key", func(service *mocks.MockGameServiceInterface) {
			service.On("CancelRun", mock.Anything, runID, "key").Return(nil, failure)
		}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service := mocks.NewMockGameServiceInterface(t)
			testCase.setup(service)
			response := iteration6Request(iteration6HandlerEngine(t, service), testCase.method, testCase.path, testCase.body, testCase.key)
			assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), "INTERNAL_ERROR")
		})
	}
}

func TestIteration6Handlers_RejectMalformedEmptyBodyOperations(t *testing.T) {
	service := mocks.NewMockGameServiceInterface(t)
	engine := iteration6HandlerEngine(t, service)
	runID := "11111111-1111-4111-8111-111111111111"
	response := iteration6Request(engine, http.MethodPost, "/v2/manager/runs/"+runID+"/qr", "{", "key")
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "INVALID_REQUEST")
}

func TestIteration6Handlers_ExplicitZeroContentLengthIsEmptyBody(t *testing.T) {
	service := mocks.NewMockGameServiceInterface(t)
	runID := "11111111-1111-4111-8111-111111111111"
	service.On("StartRun", mock.Anything, runID, "key").Return(&messages.ManagerRunResponseDTO{ID: runID}, nil).Once()
	request := httptest.NewRequest(http.MethodPost, "/v2/manager/runs/"+runID+"/start", strings.NewReader("{"))
	request.Header.Set("Content-Length", "0")
	request.Header.Set("Idempotency-Key", "key")
	response := httptest.NewRecorder()
	iteration6HandlerEngine(t, service).ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestIteration6Handlers_RejectUnknownQueryOnEveryOperation(t *testing.T) {
	service := mocks.NewMockGameServiceInterface(t)
	engine := iteration6HandlerEngine(t, service)
	runID := "11111111-1111-4111-8111-111111111111"
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v2/games?unexpected=1"},
		{http.MethodGet, "/v2/games/game?unexpected=1"},
		{http.MethodGet, "/v2/rankings?scope=individual&unexpected=1"},
		{http.MethodGet, "/v2/game/overview?unexpected=1"},
		{http.MethodGet, "/v2/activity-runs/current?unexpected=1"},
		{http.MethodGet, "/v2/participations/current?unexpected=1"},
		{http.MethodPost, "/v2/qr/validate?unexpected=1"},
		{http.MethodGet, "/v2/manager/game-overview?unexpected=1"},
		{http.MethodGet, "/v2/manager/runs/" + runID + "?unexpected=1"},
		{http.MethodPost, "/v2/manager/runs?unexpected=1"},
		{http.MethodPost, "/v2/manager/runs/" + runID + "/qr?unexpected=1"},
		{http.MethodPost, "/v2/manager/runs/" + runID + "/start?unexpected=1"},
		{http.MethodPost, "/v2/manager/runs/" + runID + "/pause?unexpected=1"},
		{http.MethodPost, "/v2/manager/runs/" + runID + "/resume?unexpected=1"},
		{http.MethodPost, "/v2/manager/runs/" + runID + "/results?unexpected=1"},
		{http.MethodPost, "/v2/manager/runs/" + runID + "/cancel?unexpected=1"},
	}
	for _, request := range paths {
		response := iteration6Request(engine, request.method, request.path, "", "")
		assert.Equal(t, http.StatusBadRequest, response.Code, request.path)
		assert.Contains(t, response.Body.String(), "INVALID_REQUEST")
	}
}
