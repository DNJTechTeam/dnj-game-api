package routers

func (r *Router) RegisterContentRoutes() {
	r.v2Group.GET("/schedule", r.handlers.ContentHandler.Schedule)
	r.v2Group.GET("/activities", r.handlers.ContentHandler.ListActivities)
	r.v2Group.GET("/activities/:activityId", r.handlers.ContentHandler.GetActivity)

	favorites := r.v2Group.Group("/users/me/favorites")
	favorites.GET("", append(r.authProtected(), r.handlers.FavoriteHandler.List)...)
	favorites.PUT("/:activityId", append(r.authProtected(), r.handlers.FavoriteHandler.Put)...)
	favorites.DELETE("/:activityId", append(r.authProtected(), r.handlers.FavoriteHandler.Delete)...)
}
