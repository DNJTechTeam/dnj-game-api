package routers

func (r *Router) RegisterTaskRoutes() {
	group := r.group.Group("/tasks")

	group.GET("", append(r.authProtected(), r.handlers.TaskHandler.List)...)
	group.GET("/:id", append(r.authProtected(), r.handlers.TaskHandler.Get)...)
	group.POST("", append(r.authProtected(), r.handlers.TaskHandler.Create)...)
	group.PUT("/:id", append(r.authProtected(), r.handlers.TaskHandler.Update)...)
	group.PATCH("/:id/status", append(r.authProtected(), r.handlers.TaskHandler.UpdateStatus)...)
	group.DELETE("/:id", append(r.authProtected(), r.handlers.TaskHandler.Delete)...)
}
