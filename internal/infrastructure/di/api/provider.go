package api

import (
	"strings"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func ProvideEngine() *gin.Engine {
	engine := gin.Default()

	engine.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Split(common.GetEnv("CORS_ALLOWED_ORIGINS"), ","),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"Set-Cookie"},
	}))

	return engine
}

func ProvideRouter(engine *gin.Engine, handlers *handlers.Handlers) *routers.Router {
	return routers.NewRouter(engine, handlers)
}
