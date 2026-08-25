package api

import (
	"log/slog"
	"os"
	"strings"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/middlewares"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func ProvideEngine() *gin.Engine {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	engine := gin.New()
	// API Gateway/Lambda supplies forwarding metadata through the adapter; the
	// direct HTTP runner must not trust arbitrary proxy headers.
	_ = engine.SetTrustedProxies(nil)
	engine.Use(middlewares.RequestObservabilityMiddleware(logger))
	engine.Use(middlewares.RecoveryMiddleware(logger))

	engine.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Split(common.GetEnv("CORS_ALLOWED_ORIGINS"), ","),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "X-CSRF-Token", "Idempotency-Key"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"Set-Cookie", "X-Request-ID"},
	}))

	return engine
}

func ProvideRouter(engine *gin.Engine, handlers *handlers.Handlers) *routers.Router {
	return routers.NewRouter(engine, handlers)
}
