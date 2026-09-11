package handlers_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	apiCookies "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/middlewares"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func identityTestEngine(t *testing.T, service *mocks.MockIdentityServiceInterface) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("SERVER_ENVIRONMENT", "test")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	engine := gin.New()
	engine.Use(middlewares.RequestObservabilityMiddleware(logger))
	handler := &handlers.IdentityHandler{IdentityService: service}
	engine.POST("/v2/auth/google", handler.Google)
	engine.POST("/v2/auth/signup", handler.SignupWithEmail)
	engine.POST("/v2/auth/signup/verify", handler.VerifyEmailSignup)
	engine.POST("/v2/auth/refresh", handler.Refresh)
	engine.POST("/v2/auth/logout", handler.Logout)
	engine.GET("/v2/auth/session", handler.Current)
	engine.PATCH("/v2/auth/onboarding", handler.CompleteOnboarding)
	return engine
}

func sessionResponse() *messages.IdentitySessionResponseDTO {
	return &messages.IdentitySessionResponseDTO{
		AccessToken: "access", RefreshToken: "refresh", CSRFToken: "csrf", TokenType: "Bearer", ExpiresIn: 900,
		OnboardingRequired: true,
		User:               &messages.IdentityUserResponseDTO{ID: 1, Email: "ana@example.com", Name: "Ana"},
	}
}

func TestIdentityHandler_GoogleContractAndCookies(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	service.On("AuthenticateGoogle", mock.Anything, mock.Anything).Return(sessionResponse(), nil)
	engine := identityTestEngine(t, service)
	request := httptest.NewRequest(http.MethodPost, "/v2/auth/google", bytes.NewBufferString(`{"idToken":"google-token"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"accessToken":"access"`)
	assert.Contains(t, recorder.Body.String(), `"refreshToken":"refresh"`)
	cookies := recorder.Result().Cookies()
	assert.Len(t, cookies, 3)
	byName := map[string]*http.Cookie{}
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
	}
	assert.True(t, byName[apiCookies.IdentityTokenName].HttpOnly)
	assert.True(t, byName[apiCookies.RefreshTokenName].HttpOnly)
	assert.False(t, byName[apiCookies.CSRFTokenName].HttpOnly)
	assert.True(t, byName[apiCookies.IdentityTokenName].Secure)
	assert.Equal(t, http.SameSiteNoneMode, byName[apiCookies.IdentityTokenName].SameSite)
	assert.Equal(t, "/v2/auth", byName[apiCookies.RefreshTokenName].Path)
}

func TestIdentityHandler_SignupWithEmailRejectsMissingEmail(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	engine := identityTestEngine(t, service)
	request := httptest.NewRequest(http.MethodPost, "/v2/auth/signup", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	service.AssertNotCalled(t, "SignupWithEmail", mock.Anything, mock.Anything)
}

func TestIdentityHandler_SignupWithEmailDelegatesToService(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	service.On("SignupWithEmail", mock.Anything, &messages.EmailSignupRequestDTO{Email: "ana@example.com"}).
		Return(&messages.EmailSignupResponseDTO{Status: "CODE_SENT"}, nil)
	engine := identityTestEngine(t, service)
	request := httptest.NewRequest(http.MethodPost, "/v2/auth/signup", bytes.NewBufferString(`{"email":"ana@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "CODE_SENT")
}

func TestIdentityHandler_VerifyEmailSignupSetsSessionCookies(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	service.On("VerifyEmailSignup", mock.Anything, &messages.VerifyEmailSignupRequestDTO{Email: "ana@example.com", Code: "042917"}).
		Return(sessionResponse(), nil)
	engine := identityTestEngine(t, service)
	request := httptest.NewRequest(
		http.MethodPost, "/v2/auth/signup/verify", bytes.NewBufferString(`{"email":"ana@example.com","code":"042917"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Len(t, recorder.Result().Cookies(), 3)
}

func TestIdentityHandler_RefreshAndLogoutRequireCSRF(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	engine := identityTestEngine(t, service)

	// when
	missingCSRF := httptest.NewRecorder()
	missingCSRFRequest := httptest.NewRequest(http.MethodPost, "/v2/auth/refresh", nil)
	missingCSRFRequest.AddCookie(&http.Cookie{Name: apiCookies.RefreshTokenName, Value: "old-refresh"})
	engine.ServeHTTP(missingCSRF, missingCSRFRequest)

	// then
	assert.Equal(t, http.StatusForbidden, missingCSRF.Code)
	assert.Contains(t, missingCSRF.Body.String(), "CSRF_INVALID")

	// given
	service.On("Refresh", mock.Anything, "old-refresh").Return(sessionResponse(), nil).Once()
	request := httptest.NewRequest(http.MethodPost, "/v2/auth/refresh", nil)
	request.Header.Set("X-CSRF-Token", "csrf-value")
	request.AddCookie(&http.Cookie{Name: apiCookies.CSRFTokenName, Value: "csrf-value"})
	request.AddCookie(&http.Cookie{Name: apiCookies.RefreshTokenName, Value: "old-refresh"})
	rotated := httptest.NewRecorder()

	// when
	engine.ServeHTTP(rotated, request)

	// then
	assert.Equal(t, http.StatusOK, rotated.Code)

	// given
	service.On("Logout", mock.Anything, "refresh").Return(nil).Once()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/v2/auth/logout", nil)
	logoutRequest.Header.Set("X-CSRF-Token", "csrf")
	logoutRequest.AddCookie(&http.Cookie{Name: apiCookies.CSRFTokenName, Value: "csrf"})
	logoutRequest.AddCookie(&http.Cookie{Name: apiCookies.RefreshTokenName, Value: "refresh"})
	logout := httptest.NewRecorder()

	// when
	engine.ServeHTTP(logout, logoutRequest)

	// then
	assert.Equal(t, http.StatusOK, logout.Code)
	assert.Contains(t, logout.Body.String(), "logged_out")
}

func TestIdentityHandler_ErrorAndOnboardingContracts(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	service.On("AuthenticateGoogle", mock.Anything, mock.Anything).
		Return(nil, appErrors.NewAPIServiceError(http.StatusUnauthorized, "INVALID_GOOGLE_TOKEN", "Token inválido.", nil)).Once()
	service.On("Current", mock.Anything).Return(&messages.CurrentSessionResponseDTO{OnboardingRequired: true}, nil).Once()
	service.On("CompleteOnboarding", mock.Anything, mock.Anything).
		Return(nil, appErrors.NewAPIServiceError(http.StatusNotFound, "GROUP_NOT_FOUND", "Grupo não encontrado.", nil)).Once()
	engine := identityTestEngine(t, service)

	// when
	google := httptest.NewRecorder()
	googleRequest := httptest.NewRequest(http.MethodPost, "/v2/auth/google", bytes.NewBufferString(`{"idToken":"invalid"}`))
	googleRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(google, googleRequest)
	current := httptest.NewRecorder()
	engine.ServeHTTP(current, httptest.NewRequest(http.MethodGet, "/v2/auth/session", nil))
	onboarding := httptest.NewRecorder()
	onboardingRequest := httptest.NewRequest(http.MethodPatch, "/v2/auth/onboarding", bytes.NewBufferString(`{"document":"52998224725","mobilePhone":"5541999990000","groupId":"99"}`))
	onboardingRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(onboarding, onboardingRequest)

	// then
	assert.Equal(t, http.StatusUnauthorized, google.Code)
	assert.Contains(t, google.Body.String(), "INVALID_GOOGLE_TOKEN")
	assert.Equal(t, http.StatusOK, current.Code)
	assert.Equal(t, http.StatusNotFound, onboarding.Code)
}

func TestIdentityHandler_RefreshAndLogoutAcceptBearerJSONBody(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	service.On("Refresh", mock.Anything, "stored-refresh").Return(sessionResponse(), nil).Once()
	service.On("Logout", mock.Anything, "stored-refresh").Return(nil).Once()
	engine := identityTestEngine(t, service)
	refreshRequest := httptest.NewRequest(http.MethodPost, "/v2/auth/refresh", bytes.NewBufferString(`{"refreshToken":"stored-refresh"}`))
	refreshRequest.Header.Set("Content-Type", "application/json")
	logoutRequest := httptest.NewRequest(http.MethodPost, "/v2/auth/logout", bytes.NewBufferString(`{"refreshToken":"stored-refresh"}`))
	logoutRequest.Header.Set("Content-Type", "application/json")

	// when
	rotated := httptest.NewRecorder()
	engine.ServeHTTP(rotated, refreshRequest)
	logout := httptest.NewRecorder()
	engine.ServeHTTP(logout, logoutRequest)

	// then
	assert.Equal(t, http.StatusOK, rotated.Code)
	assert.Contains(t, rotated.Body.String(), `"refreshToken":"refresh"`)
	assert.Empty(t, rotated.Result().Cookies(), "bearer clients must not receive session cookies")
	assert.Equal(t, http.StatusOK, logout.Code)
	assert.Contains(t, logout.Body.String(), "logged_out")
	assert.Empty(t, logout.Result().Cookies())
}

func TestIdentityHandler_BearerRefreshFailurePropagates401WithoutCookies(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	service.On("Refresh", mock.Anything, "revoked").
		Return(nil, appErrors.NewAPIServiceError(http.StatusUnauthorized, "REFRESH_TOKEN_REUSE", "Sessão inválida.", nil)).Once()
	engine := identityTestEngine(t, service)
	request := httptest.NewRequest(http.MethodPost, "/v2/auth/refresh", bytes.NewBufferString(`{"refreshToken":"revoked"}`))
	request.Header.Set("Content-Type", "application/json")

	// when
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "REFRESH_TOKEN_REUSE")
	assert.Empty(t, recorder.Result().Cookies())
}

func TestIdentityHandler_RefreshAndLogoutWithoutAnyToken(t *testing.T) {
	// given
	service := mocks.NewMockIdentityServiceInterface(t)
	service.On("Refresh", mock.Anything, "").
		Return(nil, appErrors.NewAPIServiceError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Sessão inválida.", nil)).Once()
	service.On("Logout", mock.Anything, "").Return(nil).Once()
	engine := identityTestEngine(t, service)
	malformed := httptest.NewRequest(http.MethodPost, "/v2/auth/refresh", bytes.NewBufferString(`{"refreshToken":`))
	malformed.Header.Set("Content-Type", "application/json")

	// when
	refresh := httptest.NewRecorder()
	engine.ServeHTTP(refresh, httptest.NewRequest(http.MethodPost, "/v2/auth/refresh", nil))
	logout := httptest.NewRecorder()
	engine.ServeHTTP(logout, httptest.NewRequest(http.MethodPost, "/v2/auth/logout", nil))
	badJSON := httptest.NewRecorder()
	engine.ServeHTTP(badJSON, malformed)

	// then
	assert.Equal(t, http.StatusUnauthorized, refresh.Code)
	assert.Contains(t, refresh.Body.String(), "INVALID_REFRESH_TOKEN")
	assert.Equal(t, http.StatusOK, logout.Code, "logout is idempotent when there is nothing to revoke")
	assert.Equal(t, http.StatusBadRequest, badJSON.Code)
	assert.Contains(t, badJSON.Body.String(), "INVALID_REQUEST")
}
