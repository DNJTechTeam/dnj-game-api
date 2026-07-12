package routers

func (r *Router) RegisterAuthRoutes() {
	group := r.group.Group("/auth")

	group.POST("/onboarding", r.handlers.AuthHandler.Onboarding)
	group.POST("/verification-code", r.handlers.AuthHandler.VerifyCode)
}
