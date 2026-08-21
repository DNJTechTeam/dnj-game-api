package routers_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/middlewares"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestHealthcheckRoutes_Contract(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("API_PREFIX", "/v1")
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	engine := gin.New()
	engine.Use(middlewares.RequestObservabilityMiddleware(logger), middlewares.RecoveryMiddleware(logger))
	router := routers.NewRouter(engine, &handlers.Handlers{HealthcheckHandler: &handlers.HealthcheckHandler{DB: db}})
	router.RegisterHealthcheckRoutes()

	t.Run("GET /v2/healthcheck", func(t *testing.T) {
		// given
		request := httptest.NewRequest(http.MethodGet, "/v2/healthcheck", nil)
		request.Header.Set("X-Request-ID", "router-healthcheck-1")
		recorder := httptest.NewRecorder()

		// when
		engine.ServeHTTP(recorder, request)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "router-healthcheck-1", recorder.Header().Get("X-Request-ID"))
		assert.JSONEq(t, `{"service":"dnj-game-api","status":"ok"}`, recorder.Body.String())
	})

	t.Run("GET /v2/readiness returns 200", func(t *testing.T) {
		// given
		mock.ExpectPing()
		request := httptest.NewRequest(http.MethodGet, "/v2/readiness", nil)
		recorder := httptest.NewRecorder()

		// when
		engine.ServeHTTP(recorder, request)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.NotEmpty(t, recorder.Header().Get("X-Request-ID"))
		assert.JSONEq(t, `{"status":"ready"}`, recorder.Body.String())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
