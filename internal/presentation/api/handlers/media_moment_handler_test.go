package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func mediaMomentHandlerEngine(
	t *testing.T,
	mediaService *mocks.MockMediaServiceInterface,
	momentService *mocks.MockMomentServiceInterface,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mediaHandler := &handlers.MediaHandler{MediaService: mediaService}
	momentHandler := &handlers.MomentHandler{MomentService: momentService}
	engine.POST("/v2/media/upload-intents", mediaHandler.CreateUploadIntent)
	engine.POST("/v2/media/:mediaAssetId/complete", mediaHandler.CompleteUpload)
	engine.GET("/v2/moments", momentHandler.List)
	engine.POST("/v2/moments", momentHandler.Create)
	engine.POST("/v2/moments/challenge", momentHandler.CreateChallenge)
	engine.POST("/v2/moments/:momentId/likes", momentHandler.ToggleLike)
	engine.GET("/v2/admin/moments/moderation", momentHandler.ListModeration)
	engine.POST("/v2/admin/moments/:momentId/moderation", momentHandler.Moderate)
	return engine
}

func mediaMomentRequest(engine http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestMediaMoments_HandlerHappyPaths(t *testing.T) {
	mediaService := mocks.NewMockMediaServiceInterface(t)
	momentService := mocks.NewMockMomentServiceInterface(t)
	key := "22222222-2222-4222-8222-222222222222"

	mediaService.On("CreateUploadIntent", mock.Anything, key, mock.AnythingOfType("*messages.CreateUploadIntentRequestDTO")).
		Return(&messages.UploadIntentResponseDTO{ID: "asset-1"}, http.StatusCreated, nil).Once()
	mediaService.On("CompleteUpload", mock.Anything, "asset-1", key).
		Return(&messages.MediaAssetResponseDTO{ID: "asset-1"}, http.StatusOK, nil).Once()
	momentService.On("List", mock.Anything, "mine", "").
		Return(&messages.MomentPageResponseDTO{Items: []messages.MomentResponseDTO{}}, nil).Once()
	momentService.On("Create", mock.Anything, key, mock.AnythingOfType("*messages.CreateMomentRequestDTO")).
		Return(&messages.MomentResponseDTO{ID: "moment-1"}, http.StatusCreated, nil).Once()
	momentService.On("ToggleLike", mock.Anything, "moment-1", key).
		Return(&messages.LikeResponseDTO{MomentID: "moment-1", Liked: true, LikesCount: 1}, nil).Once()
	momentService.On("ListModeration", mock.Anything, "general", uint64(0)).
		Return(&messages.ModerationPageResponseDTO{Data: []messages.ModerationMomentResponseDTO{}}, nil).Once()
	momentService.On("Moderate", mock.Anything, "moment-1", key, mock.AnythingOfType("*messages.ModerationRequestDTO")).
		Return(&messages.ModerationResponseDTO{MomentID: "moment-1"}, nil).Once()

	engine := mediaMomentHandlerEngine(t, mediaService, momentService)

	responses := []*httptest.ResponseRecorder{
		mediaMomentRequest(engine, http.MethodPost, "/v2/media/upload-intents", `{"contentType":"image/jpeg","bytes":100,"checksumSha256":"abc"}`, map[string]string{"Idempotency-Key": key}),
		mediaMomentRequest(engine, http.MethodPost, "/v2/media/asset-1/complete", "", map[string]string{"Idempotency-Key": key}),
		mediaMomentRequest(engine, http.MethodGet, "/v2/moments?scope=mine", "", nil),
		mediaMomentRequest(engine, http.MethodPost, "/v2/moments", `{"mediaAssetId":"asset-1","publishConsent":true}`, map[string]string{"Idempotency-Key": key}),
		mediaMomentRequest(engine, http.MethodPost, "/v2/moments/moment-1/likes", "", map[string]string{"Idempotency-Key": key}),
		mediaMomentRequest(engine, http.MethodGet, "/v2/admin/moments/moderation?queue=general", "", nil),
		mediaMomentRequest(engine, http.MethodPost, "/v2/admin/moments/moment-1/moderation", `{"action":"deny_points"}`, map[string]string{"Idempotency-Key": key}),
	}
	for i, r := range responses {
		assert.Lessf(t, r.Code, 300, "response %d: %s", i, r.Body.String())
	}
}

func TestMediaMoments_CreateChallengeAcceptsOnlyChallengePayload(t *testing.T) {
	momentService := mocks.NewMockMomentServiceInterface(t)
	key := "22222222-2222-4222-8222-222222222222"
	momentService.On(
		"Create",
		mock.Anything,
		key,
		mock.MatchedBy(func(request *messages.CreateMomentRequestDTO) bool {
			return request.MediaAssetID == "asset-1" && request.PublishConsent && request.ChallengeMode && request.ParticipationID == nil
		}),
	).Return(&messages.MomentResponseDTO{ID: "moment-1", Origin: "challenge"}, http.StatusCreated, nil).Once()

	engine := mediaMomentHandlerEngine(t, mocks.NewMockMediaServiceInterface(t), momentService)

	created := mediaMomentRequest(engine, http.MethodPost, "/v2/moments/challenge", `{"mediaAssetId":"asset-1","publishConsent":true}`, map[string]string{"Idempotency-Key": key})
	assert.Equal(t, http.StatusCreated, created.Code)
	assert.Contains(t, created.Body.String(), `"origin":"challenge"`)

	for _, body := range []string{
		`{"mediaAssetId":"asset-1","publishConsent":true,"participationId":"p-1"}`,
		`{"mediaAssetId":"asset-1","publishConsent":true,"challengeId":"c-1"}`,
	} {
		rejected := mediaMomentRequest(engine, http.MethodPost, "/v2/moments/challenge", body, nil)
		assert.Equal(t, http.StatusBadRequest, rejected.Code)
		assert.Contains(t, rejected.Body.String(), "INVALID_REQUEST")
	}
}

func TestMediaMoments_HandlerErrorBranches(t *testing.T) {
	mediaService := mocks.NewMockMediaServiceInterface(t)
	momentService := mocks.NewMockMomentServiceInterface(t)
	engine := mediaMomentHandlerEngine(t, mediaService, momentService)

	// Unknown/repeated query params are rejected before reaching the service.
	r := mediaMomentRequest(engine, http.MethodPost, "/v2/media/upload-intents?extra=1", `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)

	// Malformed / mass-assignment JSON body is rejected.
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/media/upload-intents", `{"contentType":"image/jpeg","bytes":100,"checksumSha256":"abc","bucket":"x"}`, nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)

	// CompleteUpload/ToggleLike do not accept a body.
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/media/asset-1/complete", `{"x":1}`, nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/moments/moment-1/likes", `{"x":1}`, nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)

	// List/ListModeration require their discriminating query param.
	r = mediaMomentRequest(engine, http.MethodGet, "/v2/moments", "", nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)
	r = mediaMomentRequest(engine, http.MethodGet, "/v2/admin/moments/moderation", "", nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)

	// Invalid page is rejected before reaching the service.
	r = mediaMomentRequest(engine, http.MethodGet, "/v2/admin/moments/moderation?queue=general&page=not-a-number", "", nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)

	// Create, ToggleLike, and Moderate accept no query parameters at all.
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/moments?scope=mine", `{"mediaAssetId":"a","publishConsent":true}`, nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/moments/moment-1/likes?x=1", "", nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/admin/moments/moment-1/moderation?x=1", `{"action":"deny_points"}`, nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)

	// Create/Moderate reject unknown fields (mass assignment).
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/moments", `{"mediaAssetId":"a","publishConsent":true,"userId":"1"}`, nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/admin/moments/moment-1/moderation", `{"action":"deny_points","reason":"x"}`, nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)

	// Service-layer API errors are propagated with their declared status/code.
	mediaService.On("CreateUploadIntent", mock.Anything, "", mock.AnythingOfType("*messages.CreateUploadIntentRequestDTO")).
		Return(nil, 0, appErrors.NewAPIServiceError(http.StatusServiceUnavailable, "MEDIA_UNAVAILABLE", "unavailable", nil)).Once()
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/media/upload-intents", `{"contentType":"image/jpeg","bytes":100,"checksumSha256":"abc"}`, nil)
	assert.Equal(t, http.StatusServiceUnavailable, r.Code)
	assert.Contains(t, r.Body.String(), "MEDIA_UNAVAILABLE")

	momentService.On("List", mock.Anything, "feed", "bad-cursor").
		Return(nil, appErrors.NewAPIServiceError(http.StatusBadRequest, "INVALID_CURSOR", "bad cursor", nil)).Once()
	r = mediaMomentRequest(engine, http.MethodGet, "/v2/moments?scope=feed&cursor=bad-cursor", "", nil)
	assert.Equal(t, http.StatusBadRequest, r.Code)
	assert.Contains(t, r.Body.String(), "INVALID_CURSOR")

	// A non-API-service error is redacted into a generic internal error, never leaked to the client.
	leaked := errors.New("raw database connection string leaked here")
	mediaService.On("CompleteUpload", mock.Anything, "asset-1", "").
		Return(nil, 0, leaked).Once()
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/media/asset-1/complete", "", nil)
	assert.Equal(t, http.StatusInternalServerError, r.Code)
	require.NotContains(t, r.Body.String(), leaked.Error())

	momentService.On("Create", mock.Anything, "", mock.AnythingOfType("*messages.CreateMomentRequestDTO")).
		Return(nil, 0, appErrors.NewAPIServiceError(http.StatusConflict, "MOMENT_ALREADY_CREATED", "conflict", nil)).Once()
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/moments", `{"mediaAssetId":"a","publishConsent":true}`, nil)
	assert.Equal(t, http.StatusConflict, r.Code)
	assert.Contains(t, r.Body.String(), "MOMENT_ALREADY_CREATED")

	momentService.On("ToggleLike", mock.Anything, "moment-1", "").
		Return(nil, appErrors.NewAPIServiceError(http.StatusNotFound, "NOT_FOUND", "not found", nil)).Once()
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/moments/moment-1/likes", "", nil)
	assert.Equal(t, http.StatusNotFound, r.Code)

	momentService.On("Moderate", mock.Anything, "moment-1", "", mock.AnythingOfType("*messages.ModerationRequestDTO")).
		Return(nil, appErrors.NewAPIServiceError(http.StatusConflict, "MODERATION_ACTION_INVALID", "invalid", nil)).Once()
	r = mediaMomentRequest(engine, http.MethodPost, "/v2/admin/moments/moment-1/moderation", `{"action":"deny_points"}`, nil)
	assert.Equal(t, http.StatusConflict, r.Code)
	assert.Contains(t, r.Body.String(), "MODERATION_ACTION_INVALID")
}
