package routers

func (r *Router) RegisterInstallationRoutes() {
	r.v2Group.GET("/spaces", r.handlers.InstallationHandler.ListSpaces)
	manager := r.v2Group.Group("/manager/activities")
	manager.POST("/:id/start", append(r.authProtected(), r.handlers.InstallationHandler.StartActivity)...)
	manager.POST("/:id/pause", append(r.authProtected(), r.handlers.InstallationHandler.PauseActivity)...)
	manager.POST("/:id/conclude", append(r.authProtected(), r.handlers.InstallationHandler.ConcludeActivity)...)
}
