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

func iteration3Engine(t *testing.T, profile *mocks.MockProfileServiceInterface, groups *mocks.MockGroupServiceInterface, invites *mocks.MockGroupInviteServiceInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	profileHandler := &handlers.ProfileHandler{ProfileService: profile}
	groupHandler := &handlers.GroupHandler{GroupService: groups}
	inviteHandler := &handlers.GroupInviteHandler{GroupInviteService: invites}
	engine.GET("/v2/users/me", profileHandler.Current)
	engine.PATCH("/v2/users/me", profileHandler.Update)
	engine.PATCH("/v2/users/me/group", groupHandler.UpdateCurrent)
	engine.POST("/v2/users/me/group", groupHandler.UpdateCurrent)
	engine.GET("/v2/groups", groupHandler.Search)
	engine.GET("/v2/groups/me", groupHandler.Current)
	engine.GET("/v2/groups/me/members", groupHandler.Members)
	engine.POST("/v2/groups/invites/consume", inviteHandler.Consume)
	engine.GET("/v2/admin/groups/:groupId/invites", inviteHandler.List)
	engine.POST("/v2/admin/groups/:groupId/invites", inviteHandler.Create)
	engine.POST("/v2/admin/groups/:groupId/invites/:inviteId/renew", inviteHandler.Renew)
	engine.DELETE("/v2/admin/groups/:groupId/invites/:inviteId", inviteHandler.Revoke)
	return engine
}

func performJSON(engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestIteration3Handlers_ProfileAndCompatibilityAlias(t *testing.T) {
	// given
	profile := mocks.NewMockProfileServiceInterface(t)
	groups := mocks.NewMockGroupServiceInterface(t)
	invites := mocks.NewMockGroupInviteServiceInterface(t)
	profileResponse := &messages.CurrentProfileResponseDTO{ID: 1, Email: "ana@example.com", Name: "Ana", DocumentMasked: "***.***.*47-25"}
	profile.On("Current", mock.Anything).Return(profileResponse, nil).Once()
	profile.On("Update", mock.Anything, mock.Anything).Return(profileResponse, nil).Once()
	groups.On("UpdateCurrent", mock.Anything, mock.Anything).Return(profileResponse, nil).Twice()
	engine := iteration3Engine(t, profile, groups, invites)

	// when
	current := performJSON(engine, http.MethodGet, "/v2/users/me", "")
	updated := performJSON(engine, http.MethodPatch, "/v2/users/me", `{"name":"Ana"}`)
	unknownField := performJSON(engine, http.MethodPatch, "/v2/users/me", `{"email":"attacker@example.com"}`)
	canonical := performJSON(engine, http.MethodPatch, "/v2/users/me/group", `{"groupId":"7"}`)
	alias := performJSON(engine, http.MethodPost, "/v2/users/me/group", `{"groupId":null}`)

	// then
	assert.Equal(t, http.StatusOK, current.Code)
	assert.NotContains(t, current.Body.String(), "documentHash")
	assert.Equal(t, http.StatusOK, updated.Code)
	assert.Equal(t, http.StatusBadRequest, unknownField.Code)
	assert.Contains(t, unknownField.Body.String(), "INVALID_REQUEST")
	assert.Equal(t, http.StatusOK, canonical.Code)
	assert.Equal(t, http.StatusOK, alias.Code)
}

func TestIteration3Handlers_GroupPaginationAndInviteLifecycle(t *testing.T) {
	// given
	profile := mocks.NewMockProfileServiceInterface(t)
	groups := mocks.NewMockGroupServiceInterface(t)
	invites := mocks.NewMockGroupInviteServiceInterface(t)
	pagination := messages.Pagination{CurrentPage: 1, HasNextPage: true, Limit: 20}
	groups.On("Search", mock.Anything, "", mock.Anything).Return(&messages.PaginatedResponse[messages.GroupSummaryDTO]{Data: []messages.GroupSummaryDTO{{ID: 1, GroupName: "Grupo A"}}, Pagination: pagination}, nil).Once()
	groups.On("Current", mock.Anything).Return(&messages.CurrentGroupResponseDTO{}, nil).Once()
	groups.On("Members", mock.Anything, mock.Anything).Return(&messages.PaginatedResponse[messages.GroupMemberResponseDTO]{Data: []messages.GroupMemberResponseDTO{}, Pagination: messages.Pagination{CurrentPage: 1, Limit: 10}}, nil).Once()
	invite := &messages.GroupInviteResponseDTO{ID: 3, GroupID: 2, Status: "ACTIVE", Code: "secret"}
	invites.On("Create", mock.Anything, uint64(2)).Return(invite, nil).Once()
	invites.On("List", mock.Anything, uint64(2), mock.Anything).Return(&messages.PaginatedResponse[messages.GroupInviteResponseDTO]{Data: []messages.GroupInviteResponseDTO{*invite}}, nil).Once()
	invites.On("Renew", mock.Anything, uint64(2), uint64(3)).Return(invite, nil).Once()
	invites.On("Revoke", mock.Anything, uint64(2), uint64(3)).Return(nil).Once()
	invites.On("Consume", mock.Anything, mock.Anything).Return(&messages.CurrentGroupResponseDTO{}, nil).Once()
	engine := iteration3Engine(t, profile, groups, invites)

	// when
	search := performJSON(engine, http.MethodGet, "/v2/groups", "")
	current := performJSON(engine, http.MethodGet, "/v2/groups/me", "")
	members := performJSON(engine, http.MethodGet, "/v2/groups/me/members?page=1", "")
	created := performJSON(engine, http.MethodPost, "/v2/admin/groups/2/invites", "")
	listed := performJSON(engine, http.MethodGet, "/v2/admin/groups/2/invites", "")
	renewed := performJSON(engine, http.MethodPost, "/v2/admin/groups/2/invites/3/renew", "")
	revoked := performJSON(engine, http.MethodDelete, "/v2/admin/groups/2/invites/3", "")
	consumed := performJSON(engine, http.MethodPost, "/v2/groups/invites/consume", `{"code":"secret"}`)

	// then
	assert.Equal(t, http.StatusOK, search.Code)
	assert.Equal(t, "1", search.Header().Get("X-Page"))
	assert.Equal(t, "true", search.Header().Get("X-Has-Next-Page"))
	assert.Equal(t, http.StatusOK, current.Code)
	assert.Equal(t, http.StatusOK, members.Code)
	assert.Equal(t, http.StatusCreated, created.Code)
	assert.Equal(t, http.StatusOK, listed.Code)
	assert.Equal(t, http.StatusCreated, renewed.Code)
	assert.Equal(t, http.StatusNoContent, revoked.Code)
	assert.Equal(t, http.StatusOK, consumed.Code)
}

func TestIteration3Handlers_PublishedErrors(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		status int
		code   string
	}{
		{name: "unauthenticated", status: 401, code: "UNAUTHENTICATED"},
		{name: "forbidden", status: 403, code: "FORBIDDEN"},
		{name: "not found", status: 404, code: "INVITE_NOT_FOUND_OR_UNAVAILABLE"},
		{name: "conflict", status: 409, code: "INVITE_UNAVAILABLE"},
		{name: "internal", status: 500, code: "INTERNAL_ERROR"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// given
			profile := mocks.NewMockProfileServiceInterface(t)
			groups := mocks.NewMockGroupServiceInterface(t)
			invites := mocks.NewMockGroupInviteServiceInterface(t)
			invites.On("Create", mock.Anything, uint64(2)).Return(nil, appErrors.NewAPIServiceError(testCase.status, testCase.code, "erro", nil)).Once()
			engine := iteration3Engine(t, profile, groups, invites)

			// when
			response := performJSON(engine, http.MethodPost, "/v2/admin/groups/2/invites", "")

			// then
			assert.Equal(t, testCase.status, response.Code)
			assert.Contains(t, response.Body.String(), testCase.code)
		})
	}
}
