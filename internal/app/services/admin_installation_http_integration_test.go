package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func adminHTTPRequest(engine http.Handler, method, path, body, token, key string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestAdminInstallationHTTP_AllLayersAllRoutesAndSecurity(t *testing.T) {
	// given
	service := setupAdminInstallationTest(t)
	admin, _ := seedAdminInstallationUser(t, "http-admin@example.com", userEntities.RoleAdmin, true)
	participant, _ := seedAdminInstallationUser(t, "http-participant@example.com", userEntities.RoleDefault, true)
	jwt := NewJwtService(TestSuite.BaseService)
	adminToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, admin)
	require.NoError(t, err)
	participantToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, participant)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	adminHandler := &handlers.AdminInstallationHandler{AdminInstallationService: service}
	router := routers.NewRouter(engine, &handlers.Handlers{AdminInstallationHandler: adminHandler})
	router.RegisterAdminInstallationRoutes()
	spaceKey := uuid.NewString()

	// when
	unauthenticated := adminHTTPRequest(engine, http.MethodGet, "/v2/admin/spaces", "", "", "")
	forbidden := adminHTTPRequest(engine, http.MethodGet, "/v2/admin/spaces", "", participantToken, "")
	createdSpace := adminHTTPRequest(engine, http.MethodPost, "/v2/admin/spaces", `{"slug":"capela","name":"Capela","mapReference":"map:capela"}`, adminToken, spaceKey)
	spaceID := decodeJSONField(t, createdSpace.Body.Bytes(), "id")
	updatedSpace := adminHTTPRequest(engine, http.MethodPatch, "/v2/admin/spaces/"+spaceID, `{"name":"Capela Central"}`, adminToken, uuid.NewString())
	retriedSpace := adminHTTPRequest(engine, http.MethodPost, "/v2/admin/spaces", `{"slug":"capela","name":"Capela","mapReference":"map:capela"}`, adminToken, spaceKey)
	listedSpaces := adminHTTPRequest(engine, http.MethodGet, "/v2/admin/spaces?page=1", "", adminToken, "")
	activityBody := `{"spaceId":"` + spaceID + `","slug":"desafio-foto","name":"Desafio Foto","description":null,"kind":"challenge","startsAt":null,"endsAt":null,"checkInPoints":10,"momentPoints":20,"cooldownSeconds":60,"allowsMoment":true}`
	createdActivity := adminHTTPRequest(engine, http.MethodPost, "/v2/admin/activities", activityBody, adminToken, uuid.NewString())
	activityID := decodeJSONField(t, createdActivity.Body.Bytes(), "id")
	listedActivities := adminHTTPRequest(engine, http.MethodGet, "/v2/admin/activities?page=1", "", adminToken, "")
	updatedActivity := adminHTTPRequest(engine, http.MethodPatch, "/v2/admin/activities/"+activityID, `{"momentPoints":25}`, adminToken, uuid.NewString())
	massAssignment := adminHTTPRequest(engine, http.MethodPatch, "/v2/admin/activities/"+activityID, `{"status":"active","admin":true}`, adminToken, uuid.NewString())
	participantID := fmt.Sprint(participant.ID)
	promoted := adminHTTPRequest(engine, http.MethodPatch, "/v2/admin/users/"+participantID+"/role", `{"role":"EVENT_MANAGER"}`, adminToken, uuid.NewString())
	listedStaff := adminHTTPRequest(engine, http.MethodGet, "/v2/admin/staff?role=EVENT_MANAGER&page=1", "", adminToken, "")
	assigned := adminHTTPRequest(engine, http.MethodPut, "/v2/admin/activities/"+activityID+"/managers/"+participantID, "", adminToken, uuid.NewString())
	listedManagers := adminHTTPRequest(engine, http.MethodGet, "/v2/admin/activities/"+activityID+"/managers?page=1", "", adminToken, "")
	removed := adminHTTPRequest(engine, http.MethodDelete, "/v2/admin/activities/"+activityID+"/managers/"+participantID, "", adminToken, uuid.NewString())

	// then
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	assert.Contains(t, unauthenticated.Body.String(), "UNAUTHENTICATED")
	assert.Equal(t, http.StatusForbidden, forbidden.Code)
	assert.Contains(t, forbidden.Body.String(), "FORBIDDEN")
	for _, response := range []*httptest.ResponseRecorder{createdSpace, retriedSpace, createdActivity} {
		assert.Equal(t, http.StatusCreated, response.Code, response.Body.String())
	}
	for _, response := range []*httptest.ResponseRecorder{updatedSpace, listedSpaces, listedActivities, updatedActivity, promoted, listedStaff, assigned, listedManagers} {
		assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}
	assert.Equal(t, http.StatusBadRequest, massAssignment.Code)
	assert.Contains(t, massAssignment.Body.String(), "INVALID_REQUEST")
	assert.Equal(t, http.StatusNoContent, removed.Code)
	assert.Contains(t, updatedSpace.Body.String(), "Capela Central")
	assert.Contains(t, retriedSpace.Body.String(), `"name":"Capela"`)
	assert.NotContains(t, retriedSpace.Body.String(), "Capela Central")
	assert.Contains(t, listedSpaces.Body.String(), `"pagination"`)
	assert.Contains(t, listedActivities.Body.String(), `"status":"draft"`)
	assert.Contains(t, updatedActivity.Body.String(), `"momentPoints":25`)
	assert.Contains(t, promoted.Body.String(), "EVENT_MANAGER")
	assert.Contains(t, listedStaff.Body.String(), "http-participant@example.com")
	assert.Contains(t, listedManagers.Body.String(), participantID)
	var auditRows []models.OperationAudit
	require.NoError(t, TestSuite.DbConn.Order("created_at ASC").Find(&auditRows).Error)
	require.Len(t, auditRows, 7)
	for _, audit := range auditRows {
		assert.NotContains(t, string(audit.Metadata), "example.com")
		assert.NotContains(t, string(audit.Metadata), "map:capela")
		assert.NotContains(t, string(audit.Metadata), "token")
	}
	var operations int64
	require.NoError(t, TestSuite.DbConn.Model(&models.AdminOperation{}).Count(&operations).Error)
	assert.Equal(t, int64(7), operations)
}

func decodeJSONField(t *testing.T, raw []byte, field string) string {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	value, ok := payload[field].(string)
	require.True(t, ok, string(raw))
	return value
}
