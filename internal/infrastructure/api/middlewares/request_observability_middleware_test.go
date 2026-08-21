package middlewares_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/middlewares"
	infraCommon "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestObservabilityMiddleware_CorrelatesResponseAndStructuredLog(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	engine := gin.New()
	engine.Use(middlewares.RequestObservabilityMiddleware(logger))
	engine.GET("/resource/:id", func(c *gin.Context) {
		assert.Equal(t, "client-request-42", infraCommon.ExtractRequestID(c.Request.Context()))
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/resource/7?token=secret", nil)
	request.Header.Set("X-Request-ID", "client-request-42")
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "client-request-42", recorder.Header().Get("X-Request-ID"))
	var event map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &event))
	assert.Equal(t, "http_request_completed", event["msg"])
	assert.Equal(t, "client-request-42", event["requestId"])
	assert.Equal(t, "/resource/:id", event["route"])
	assert.NotContains(t, logs.String(), "secret")
}

func TestRequestObservabilityMiddleware_ReplacesUnsafeRequestID(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middlewares.RequestObservabilityMiddleware(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))))
	engine.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "unsafe request id\nforged")
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	requestID := recorder.Header().Get("X-Request-ID")
	assert.Len(t, requestID, 32)
	assert.NotContains(t, requestID, "unsafe")
}

func TestRecoveryMiddleware_ReturnsUniformCorrelatedError(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	engine := gin.New()
	engine.Use(middlewares.RequestObservabilityMiddleware(logger), middlewares.RecoveryMiddleware(logger))
	engine.GET("/panic", func(_ *gin.Context) { panic("sensitive panic") })
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"INTERNAL_ERROR"`)
	assert.Contains(t, recorder.Body.String(), recorder.Header().Get("X-Request-ID"))
	assert.NotContains(t, logs.String(), "sensitive panic")
}
