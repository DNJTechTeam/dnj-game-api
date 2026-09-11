package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestProvideEngine_CORSAllowsBearerClients(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com,http://localhost:3000")
	engine := ProvideEngine()
	engine.GET("/v2/auth/session", func(c *gin.Context) { c.Status(http.StatusOK) })
	preflight := httptest.NewRequest(http.MethodOptions, "/v2/auth/session", nil)
	preflight.Header.Set("Origin", "http://localhost:3000")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "authorization,content-type,idempotency-key")
	actual := httptest.NewRequest(http.MethodGet, "/v2/auth/session", nil)
	actual.Header.Set("Origin", "https://app.example.com")
	denied := httptest.NewRequest(http.MethodGet, "/v2/auth/session", nil)
	denied.Header.Set("Origin", "https://evil.example.com")

	// when
	preflightRecorder := httptest.NewRecorder()
	engine.ServeHTTP(preflightRecorder, preflight)
	actualRecorder := httptest.NewRecorder()
	engine.ServeHTTP(actualRecorder, actual)
	deniedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(deniedRecorder, denied)

	// then
	assert.Equal(t, http.StatusNoContent, preflightRecorder.Code)
	assert.Equal(t, "http://localhost:3000", preflightRecorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, preflightRecorder.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key")
	assert.Contains(t, preflightRecorder.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, preflightRecorder.Header().Get("Access-Control-Allow-Methods"), "PATCH")
	assert.Equal(t, http.StatusOK, actualRecorder.Code)
	assert.Equal(t, "https://app.example.com", actualRecorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, http.StatusForbidden, deniedRecorder.Code)
	assert.Empty(t, deniedRecorder.Header().Get("Access-Control-Allow-Origin"))
}
