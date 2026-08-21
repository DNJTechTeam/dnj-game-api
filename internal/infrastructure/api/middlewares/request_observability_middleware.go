package middlewares

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	infraCommon "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func newRequestID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(value)
}

// RequestObservabilityMiddleware correlates every response and emits one
// structured completion event. It deliberately excludes headers, query
// strings, bodies and client IPs to keep credentials and personal data out of
// logs.
func RequestObservabilityMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if !validRequestID.MatchString(requestID) {
			requestID = newRequestID()
		}

		ctx := context.WithValue(c.Request.Context(), infraCommon.RequestIDContextKey, requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Header(requestIDHeader, requestID)
		startedAt := time.Now()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		logger.InfoContext(ctx, "http_request_completed",
			"requestId", requestID,
			"method", c.Request.Method,
			"route", route,
			"status", c.Writer.Status(),
			"latencyMs", time.Since(startedAt).Milliseconds(),
			"responseBytes", c.Writer.Size(),
		)
	}
}

func RecoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		requestID := infraCommon.ExtractRequestID(c.Request.Context())
		logger.ErrorContext(c.Request.Context(), "http_request_panic",
			"requestId", requestID,
			"method", c.Request.Method,
			"route", c.FullPath(),
		)
		handlers.ResponseAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.", nil)
	})
}
