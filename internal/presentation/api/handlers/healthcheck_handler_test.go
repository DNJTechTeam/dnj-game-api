package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	infraCommon "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testContext(method string, path string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(method, path, nil)
	request = request.WithContext(context.WithValue(request.Context(), infraCommon.RequestIDContextKey, "request-123"))
	ctx.Request = request
	return ctx, recorder
}

func TestHealthcheckHandler_GetV2(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	ctx, recorder := testContext(http.MethodGet, "/v2/healthcheck")
	handler := handlers.HealthcheckHandler{}

	// when
	handler.GetV2(ctx)

	// then
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"service":"dnj-game-api","status":"ok"}`, recorder.Body.String())
}

func TestHealthcheckHandler_Ready(t *testing.T) {
	t.Run("returns ready when database responds", func(t *testing.T) {
		// given
		sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer sqlDB.Close()
		db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
		require.NoError(t, err)
		mock.ExpectPing()
		ctx, recorder := testContext(http.MethodGet, "/v2/readiness")

		// when
		(&handlers.HealthcheckHandler{DB: db}).Ready(ctx)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"status":"ready"}`, recorder.Body.String())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns correlated error when database is unavailable", func(t *testing.T) {
		// given
		ctx, recorder := testContext(http.MethodGet, "/v2/readiness")

		// when
		(&handlers.HealthcheckHandler{}).Ready(ctx)

		// then
		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
		assert.JSONEq(t, `{"code":"DEPENDENCY_UNAVAILABLE","message":"Database is unavailable.","requestId":"request-123"}`, recorder.Body.String())
	})
}
