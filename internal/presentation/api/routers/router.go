package routers

import (
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/api/middlewares"
	infraCommon "github.com/scienceandcode/nucleus-api/internal/infrastructure/common"
	"github.com/scienceandcode/nucleus-api/internal/presentation/api/handlers"

	"github.com/gin-gonic/gin"
)

type Router struct {
	engine   *gin.Engine
	handlers *handlers.Handlers
	group    *gin.RouterGroup
}

func NewRouter(engine *gin.Engine, handlers *handlers.Handlers) *Router {
	return &Router{
		engine:   engine,
		handlers: handlers,
		group:    engine.Group(infraCommon.GetEnv("API_PREFIX")),
	}
}

// authProtected returns the middleware chain for routes that require a valid,
// fully-onboarded user (email confirmed + password set). Extend this chain
// when you add a richer authorization model (roles, scopes, ...).
func (r *Router) authProtected() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middlewares.AuthenticationMiddleware(nil),
	}
}

func (r *Router) RegisterRoutes() *gin.Engine {
	r.RegisterHealthcheckRoutes()
	r.RegisterAuthRoutes()
	r.RegisterTaskRoutes()

	return r.engine
}
