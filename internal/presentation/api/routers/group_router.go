package routers

func (r *Router) RegisterGroupRoutes() {
	group := r.group.Group("/groups")

	group.GET("", append(r.authProtected(), r.handlers.GroupHandler.Search)...)
}
