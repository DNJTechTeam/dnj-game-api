package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaMomentsHTTP_MiddlewareHandlerServiceRepositoryAndDatabase(t *testing.T) {
	mediaService, momentService, storage := setupMediaMomentServices(t)
	participant, _ := seedMediaMomentUser(t, "moment-http@example.com", userEntities.RoleDefault, true)
	admin, _ := seedMediaMomentUser(t, "moment-http-admin@example.com", userEntities.RoleAdmin, true)
	jwt := NewJwtService(TestSuite.BaseService)
	participantToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, participant)
	require.NoError(t, err)
	adminToken, err := jwt.GenerateIdentityToken(TestSuite.Ctx, admin)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{
		MediaHandler:  &handlers.MediaHandler{MediaService: mediaService},
		MomentHandler: &handlers.MomentHandler{MomentService: momentService},
	})
	router.RegisterMediaMomentRoutes()

	body := mediaMomentImage(t, "image/png")
	intentBody := fmt.Sprintf(
		`{"contentType":"image/png","bytes":%d,"checksumSha256":"%s"}`,
		len(body),
		checksum(body),
	)
	intentResponse := adminHTTPRequest(
		engine,
		http.MethodPost,
		"/v2/media/upload-intents",
		intentBody,
		participantToken,
		uuid.NewString(),
	)
	require.Equal(t, http.StatusCreated, intentResponse.Code, intentResponse.Body.String())
	assert.Equal(t, "private, no-store", intentResponse.Header().Get("Cache-Control"))
	var intent messages.UploadIntentResponseDTO
	require.NoError(t, json.Unmarshal(intentResponse.Body.Bytes(), &intent))
	storage.staged[intent.ID] = body

	completeResponse := adminHTTPRequest(
		engine,
		http.MethodPost,
		"/v2/media/"+intent.ID+"/complete",
		"",
		participantToken,
		uuid.NewString(),
	)
	require.Equal(t, http.StatusOK, completeResponse.Code, completeResponse.Body.String())

	createResponse := adminHTTPRequest(
		engine,
		http.MethodPost,
		"/v2/moments",
		`{"mediaAssetId":"`+intent.ID+`","publishConsent":true}`,
		participantToken,
		uuid.NewString(),
	)
	require.Equal(t, http.StatusCreated, createResponse.Code, createResponse.Body.String())
	var moment messages.MomentResponseDTO
	require.NoError(t, json.Unmarshal(createResponse.Body.Bytes(), &moment))
	assert.Equal(t, "free", moment.Origin)
	assert.Zero(t, moment.PointsAwarded)

	feed := adminHTTPRequest(engine, http.MethodGet, "/v2/moments?scope=feed", "", participantToken, "")
	like := adminHTTPRequest(
		engine,
		http.MethodPost,
		"/v2/moments/"+moment.ID+"/likes",
		"",
		participantToken,
		uuid.NewString(),
	)
	queue := adminHTTPRequest(
		engine,
		http.MethodGet,
		"/v2/admin/moments/moderation?queue=general&page=1",
		"",
		adminToken,
		"",
	)
	deleted := adminHTTPRequest(
		engine,
		http.MethodPost,
		"/v2/admin/moments/"+moment.ID+"/moderation",
		`{"action":"delete_photo"}`,
		adminToken,
		uuid.NewString(),
	)
	for name, response := range map[string]int{
		"feed": feed.Code, "like": like.Code, "queue": queue.Code, "delete": deleted.Code,
	} {
		assert.Equal(t, http.StatusOK, response, name)
	}
	assert.Contains(t, feed.Body.String(), moment.ID)
	assert.Contains(t, like.Body.String(), `"liked":true`)
	assert.Contains(t, deleted.Body.String(), `"photoStatus":"deleted"`)

	unauthenticated := adminHTTPRequest(engine, http.MethodGet, "/v2/moments?scope=feed", "", "", "")
	forbidden := adminHTTPRequest(
		engine,
		http.MethodGet,
		"/v2/admin/moments/moderation?queue=general",
		"",
		participantToken,
		"",
	)
	repeatedScope := adminHTTPRequest(
		engine,
		http.MethodGet,
		"/v2/moments?scope=feed&scope=mine",
		"",
		participantToken,
		"",
	)
	unknownField := adminHTTPRequest(
		engine,
		http.MethodPost,
		"/v2/moments",
		`{"mediaAssetId":"`+intent.ID+`","publishConsent":true,"userId":"1"}`,
		participantToken,
		uuid.NewString(),
	)
	likeBody := adminHTTPRequest(
		engine,
		http.MethodPost,
		"/v2/moments/"+moment.ID+"/likes",
		`{}`,
		participantToken,
		uuid.NewString(),
	)
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	assert.Equal(t, http.StatusForbidden, forbidden.Code)
	assert.Equal(t, http.StatusBadRequest, repeatedScope.Code)
	assert.Equal(t, http.StatusBadRequest, unknownField.Code)
	assert.Equal(t, http.StatusBadRequest, likeBody.Code)
}
