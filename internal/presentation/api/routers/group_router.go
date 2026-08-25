package routers

func (r *Router) RegisterGroupRoutes() {
	legacy := r.group.Group("/groups")
	legacy.GET("", append(r.authProtected(), r.handlers.GroupHandler.Search)...)

	group := r.v2Group.Group("/groups")
	group.GET("", append(r.authProtected(), r.handlers.GroupHandler.Search)...)
	group.GET("/me", append(r.authProtected(), r.handlers.GroupHandler.Current)...)
	group.GET("/me/members", append(r.authProtected(), r.handlers.GroupHandler.Members)...)
	group.POST("/invites/consume", append(r.authProtected(), r.handlers.GroupInviteHandler.Consume)...)

	admin := r.v2Group.Group("/admin/groups")
	admin.GET("/:groupId/invites", append(r.authProtected(), r.handlers.GroupInviteHandler.List)...)
	admin.POST("/:groupId/invites", append(r.authProtected(), r.handlers.GroupInviteHandler.Create)...)
	admin.POST("/:groupId/invites/:inviteId/renew", append(r.authProtected(), r.handlers.GroupInviteHandler.Renew)...)
	admin.DELETE("/:groupId/invites/:inviteId", append(r.authProtected(), r.handlers.GroupInviteHandler.Revoke)...)
}
