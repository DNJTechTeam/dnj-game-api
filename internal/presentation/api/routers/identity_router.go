package routers

func (r *Router) RegisterIdentityRoutes() {
	group := r.v2Group.Group("/auth")
	group.POST("/google", r.handlers.IdentityHandler.Google)
	group.POST("/signup", r.handlers.IdentityHandler.SignupWithEmail)
	group.POST("/signup/verify", r.handlers.IdentityHandler.VerifyEmailSignup)
	group.POST("/refresh", r.handlers.IdentityHandler.Refresh)
	group.POST("/logout", r.handlers.IdentityHandler.Logout)
	group.GET("/session", append(r.authProtected(), r.handlers.IdentityHandler.Current)...)
	group.PATCH("/onboarding", append(r.authProtected(), r.handlers.IdentityHandler.CompleteOnboarding)...)
}
