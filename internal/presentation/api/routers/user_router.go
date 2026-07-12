package routers

func (r *Router) RegisterUserRoutes() {
	group := r.group.Group("/users")

	group.POST("/:id/update-group", append(r.authProtected(), r.handlers.UserHandler.UpdateGroup)...)
}
