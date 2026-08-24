package routers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/middlewares"
	infraCommon "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"

	"github.com/gin-gonic/gin"
)

type Router struct {
	engine   *gin.Engine
	handlers *handlers.Handlers
	group    *gin.RouterGroup
	v2Group  *gin.RouterGroup
}

func NewRouter(engine *gin.Engine, handlers *handlers.Handlers) *Router {
	return &Router{
		engine:   engine,
		handlers: handlers,
		group:    engine.Group(infraCommon.GetEnv("API_PREFIX")),
		v2Group:  engine.Group("/v2"),
	}
}

// authProtected returns the middleware chain for routes that require a valid
// identity token. Extend this chain when you add a richer authorization
// model (roles, scopes, ...).
func (r *Router) authProtected() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middlewares.AuthenticationMiddleware(),
	}
}

func (r *Router) RegisterRoutes() *gin.Engine {
	r.RegisterHealthcheckRoutes()
	r.RegisterAuthRoutes()
	r.RegisterIdentityRoutes()
	r.RegisterProfileRoutes()
	r.RegisterSubscriptionRoutes()
	r.RegisterGroupRoutes()
	r.RegisterUserRoutes()
	r.RegisterTaskRoutes()
	r.RegisterInstallationRoutes()

	return r.engine
}
