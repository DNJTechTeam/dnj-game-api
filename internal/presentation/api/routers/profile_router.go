package routers

func (r *Router) RegisterProfileRoutes() {
	users := r.v2Group.Group("/users")
	users.GET("/me", append(r.authProtected(), r.handlers.ProfileHandler.Current)...)
	users.PATCH("/me", append(r.authProtected(), r.handlers.ProfileHandler.Update)...)
	users.PATCH("/me/group", append(r.authProtected(), r.handlers.GroupHandler.UpdateCurrent)...)
	users.POST("/me/group", append(r.authProtected(), r.handlers.GroupHandler.UpdateCurrent)...)
}
