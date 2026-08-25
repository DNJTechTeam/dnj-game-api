package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func adminInstallationEngine(t *testing.T, service *mocks.MockAdminInstallationServiceInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := &handlers.AdminInstallationHandler{AdminInstallationService: service}
	engine.GET("/v2/admin/spaces", handler.ListSpaces)
	engine.POST("/v2/admin/spaces", handler.CreateSpace)
	engine.PATCH("/v2/admin/spaces/:spaceId", handler.UpdateSpace)
	engine.GET("/v2/admin/activities", handler.ListActivities)
	engine.POST("/v2/admin/activities", handler.CreateActivity)
	engine.PATCH("/v2/admin/activities/:activityId", handler.UpdateActivity)
	engine.GET("/v2/admin/staff", handler.ListStaff)
	engine.PATCH("/v2/admin/users/:userId/role", handler.UpdateUserRole)
	engine.GET("/v2/admin/activities/:activityId/managers", handler.ListManagers)
	engine.PUT("/v2/admin/activities/:activityId/managers/:userId", handler.AssignManager)
	engine.DELETE("/v2/admin/activities/:activityId/managers/:userId", handler.RemoveManager)
	return engine
}

func performAdminJSON(engine *gin.Engine, method, path, body, key string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestAdminInstallationHandlers_AllSuccessfulOperations(t *testing.T) {
	// given
	service := mocks.NewMockAdminInstallationServiceInterface(t)
	space := &messages.SpaceResponseDTO{ID: "11111111-1111-4111-8111-111111111111", Slug: "capela", Name: "Capela"}
	activity := &messages.AdminActivityResponseDTO{ID: "22222222-2222-4222-8222-222222222222", Slug: "desafio", Name: "Desafio", Kind: "challenge", Status: "draft"}
	staff := messages.AdminStaffResponseDTO{ID: 7, Name: "Gestor", Role: "EVENT_MANAGER", OnboardingComplete: true}
	assignment := &messages.AdminManagerAssignmentResponseDTO{ActivityID: activity.ID, UserID: 7}
	service.On("ListSpaces", mock.Anything, mock.MatchedBy(func(filter *messages.ListAdminSpacesFilterDTO) bool { return filter.GetPage() == 1 })).Return(&messages.PaginatedResponse[messages.SpaceResponseDTO]{Data: []messages.SpaceResponseDTO{*space}, Pagination: messages.Pagination{CurrentPage: 2, Limit: 20}}, nil).Once()
	service.On("CreateSpace", mock.Anything, "key", mock.Anything).Return(space, nil).Once()
	service.On("UpdateSpace", mock.Anything, space.ID, "key", mock.Anything).Return(space, nil).Once()
	service.On("ListActivities", mock.Anything, mock.Anything).Return(&messages.PaginatedResponse[messages.AdminActivityResponseDTO]{Data: []messages.AdminActivityResponseDTO{*activity}, Pagination: messages.Pagination{CurrentPage: 1, Limit: 20}}, nil).Once()
	service.On("CreateActivity", mock.Anything, "key", mock.Anything).Return(activity, nil).Once()
	service.On("UpdateActivity", mock.Anything, activity.ID, "key", mock.Anything).Return(activity, nil).Once()
	service.On("ListStaff", mock.Anything, mock.MatchedBy(func(filter *messages.ListAdminStaffFilterDTO) bool { return filter.Role == "EVENT_MANAGER" })).Return(&messages.PaginatedResponse[messages.AdminStaffResponseDTO]{Data: []messages.AdminStaffResponseDTO{staff}, Pagination: messages.Pagination{CurrentPage: 1, Limit: 20}}, nil).Once()
	service.On("UpdateUserRole", mock.Anything, "7", "key", mock.Anything).Return(&messages.AdminUserRoleResponseDTO{ID: 7, Role: "EVENT_MANAGER"}, nil).Once()
	service.On("ListManagers", mock.Anything, activity.ID, mock.Anything).Return(&messages.PaginatedResponse[messages.AdminStaffResponseDTO]{Data: []messages.AdminStaffResponseDTO{staff}, Pagination: messages.Pagination{CurrentPage: 1, Limit: 20}}, nil).Once()
	service.On("AssignManager", mock.Anything, activity.ID, "7", "key").Return(assignment, nil).Once()
	service.On("RemoveManager", mock.Anything, activity.ID, "7", "key").Return(nil).Once()
	engine := adminInstallationEngine(t, service)
	activityBody := `{"slug":"desafio","name":"Desafio","kind":"challenge","checkInPoints":0,"momentPoints":0,"cooldownSeconds":0,"allowsMoment":false}`

	// when
	responses := []*httptest.ResponseRecorder{
		performAdminJSON(engine, http.MethodGet, "/v2/admin/spaces?page=2", "", ""),
		performAdminJSON(engine, http.MethodPost, "/v2/admin/spaces", `{"slug":"capela","name":"Capela","mapReference":null}`, "key"),
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/spaces/"+space.ID, `{"name":"Capela"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/activities", "", ""),
		performAdminJSON(engine, http.MethodPost, "/v2/admin/activities", activityBody, "key"),
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/activities/"+activity.ID, `{"status":"archived"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/staff?role=EVENT_MANAGER", "", ""),
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/users/7/role", `{"role":"EVENT_MANAGER"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/activities/"+activity.ID+"/managers", "", ""),
		performAdminJSON(engine, http.MethodPut, "/v2/admin/activities/"+activity.ID+"/managers/7", "", "key"),
		performAdminJSON(engine, http.MethodDelete, "/v2/admin/activities/"+activity.ID+"/managers/7", "", "key"),
	}

	// then
	expectedStatuses := []int{200, 201, 200, 200, 201, 200, 200, 200, 200, 200, 204}
	for index, response := range responses {
		assert.Equal(t, expectedStatuses[index], response.Code, response.Body.String())
	}
	assert.Contains(t, responses[0].Body.String(), `"pagination"`)
	assert.Contains(t, responses[6].Body.String(), `"EVENT_MANAGER"`)
}

func TestAdminInstallationHandlers_StrictBodiesPaginationAndErrors(t *testing.T) {
	// given
	service := mocks.NewMockAdminInstallationServiceInterface(t)
	service.On("CreateSpace", mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.NewAPIServiceError(409, "SLUG_ALREADY_EXISTS", "duplicado", nil)).Once()
	service.On("AssignManager", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.NewAPIServiceError(404, "NOT_FOUND", "ausente", nil)).Once()
	service.On("RemoveManager", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(appErrors.InternalError).Once()
	engine := adminInstallationEngine(t, service)

	// when
	unknownSpace := performAdminJSON(engine, http.MethodPost, "/v2/admin/spaces", `{"slug":"capela","name":"Capela","role":"ADMIN"}`, "key")
	unknownActivity := performAdminJSON(engine, http.MethodPost, "/v2/admin/activities", `{"slug":"a","name":"A","kind":"live","checkInPoints":0,"momentPoints":0,"cooldownSeconds":0,"allowsMoment":false,"eventId":"x"}`, "key")
	unknownPatch := performAdminJSON(engine, http.MethodPatch, "/v2/admin/activities/22222222-2222-4222-8222-222222222222", `{"started":true}`, "key")
	unknownRole := performAdminJSON(engine, http.MethodPatch, "/v2/admin/users/7/role", `{"role":"EVENT_MANAGER","admin":true}`, "key")
	invalidJSON := performAdminJSON(engine, http.MethodPatch, "/v2/admin/spaces/11111111-1111-4111-8111-111111111111", `{`, "key")
	invalidPage := performAdminJSON(engine, http.MethodGet, "/v2/admin/spaces?page=0", "", "")
	invalidPageText := performAdminJSON(engine, http.MethodGet, "/v2/admin/activities?page=x", "", "")
	conflict := performAdminJSON(engine, http.MethodPost, "/v2/admin/spaces", `{"slug":"capela","name":"Capela"}`, "key")
	notFound := performAdminJSON(engine, http.MethodPut, "/v2/admin/activities/x/managers/7", "", "key")
	internal := performAdminJSON(engine, http.MethodDelete, "/v2/admin/activities/x/managers/7", "", "key")

	// then
	for _, response := range []*httptest.ResponseRecorder{unknownSpace, unknownActivity, unknownPatch, unknownRole, invalidJSON, invalidPage, invalidPageText} {
		assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "INVALID_REQUEST")
	}
	assert.Equal(t, http.StatusConflict, conflict.Code)
	assert.Contains(t, conflict.Body.String(), "SLUG_ALREADY_EXISTS")
	assert.Equal(t, http.StatusNotFound, notFound.Code)
	assert.Equal(t, http.StatusInternalServerError, internal.Code)
}

func TestAdminInstallationHandlers_ErrorsFromEveryRemainingServiceMethod(t *testing.T) {
	// given
	service := mocks.NewMockAdminInstallationServiceInterface(t)
	serviceError := appErrors.NewAPIServiceError(http.StatusForbidden, "FORBIDDEN", "negado", nil)
	service.On("UpdateSpace", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, serviceError).Once()
	service.On("ListActivities", mock.Anything, mock.Anything).Return(nil, serviceError).Once()
	service.On("CreateActivity", mock.Anything, mock.Anything, mock.Anything).Return(nil, serviceError).Once()
	service.On("UpdateActivity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, serviceError).Once()
	service.On("ListStaff", mock.Anything, mock.Anything).Return(nil, serviceError).Once()
	service.On("UpdateUserRole", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, serviceError).Once()
	service.On("ListManagers", mock.Anything, mock.Anything, mock.Anything).Return(nil, serviceError).Once()
	engine := adminInstallationEngine(t, service)
	activityBody := `{"slug":"a","name":"A","kind":"live","checkInPoints":0,"momentPoints":0,"cooldownSeconds":0,"allowsMoment":false}`

	// when
	responses := []*httptest.ResponseRecorder{
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/spaces/id", `{"name":"Nome"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/activities", "", ""),
		performAdminJSON(engine, http.MethodPost, "/v2/admin/activities", activityBody, "key"),
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/activities/id", `{"name":"Nome"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/staff?role=EVENT_MANAGER", "", ""),
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/users/7/role", `{"role":"DEFAULT"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/activities/id/managers", "", ""),
	}

	// then
	for _, response := range responses {
		assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "FORBIDDEN")
	}
}

func TestAdminInstallationHandlers_EveryPublishedOperationMapsInternalError(t *testing.T) {
	// given
	service := mocks.NewMockAdminInstallationServiceInterface(t)
	service.On("ListSpaces", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("CreateSpace", mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("UpdateSpace", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("ListActivities", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("CreateActivity", mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("UpdateActivity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("ListStaff", mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("UpdateUserRole", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("ListManagers", mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("AssignManager", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, appErrors.InternalError).Once()
	service.On("RemoveManager", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(appErrors.InternalError).Once()
	engine := adminInstallationEngine(t, service)
	activityBody := `{"slug":"a","name":"A","kind":"live","checkInPoints":0,"momentPoints":0,"cooldownSeconds":0,"allowsMoment":false}`

	// when
	responses := []*httptest.ResponseRecorder{
		performAdminJSON(engine, http.MethodGet, "/v2/admin/spaces", "", ""),
		performAdminJSON(engine, http.MethodPost, "/v2/admin/spaces", `{"slug":"a","name":"A"}`, "key"),
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/spaces/id", `{"name":"A"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/activities", "", ""),
		performAdminJSON(engine, http.MethodPost, "/v2/admin/activities", activityBody, "key"),
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/activities/id", `{"name":"A"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/staff?role=EVENT_MANAGER", "", ""),
		performAdminJSON(engine, http.MethodPatch, "/v2/admin/users/7/role", `{"role":"DEFAULT"}`, "key"),
		performAdminJSON(engine, http.MethodGet, "/v2/admin/activities/id/managers", "", ""),
		performAdminJSON(engine, http.MethodPut, "/v2/admin/activities/id/managers/7", "", "key"),
		performAdminJSON(engine, http.MethodDelete, "/v2/admin/activities/id/managers/7", "", "key"),
	}

	// then
	for _, response := range responses {
		assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "INTERNAL_ERROR")
	}
}
