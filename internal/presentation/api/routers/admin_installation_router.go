package routers

func (r *Router) RegisterAdminInstallationRoutes() {
	admin := r.v2Group.Group("/admin", r.authProtected()...)
	admin.GET("/spaces", r.handlers.AdminInstallationHandler.ListSpaces)
	admin.POST("/spaces", r.handlers.AdminInstallationHandler.CreateSpace)
	admin.PATCH("/spaces/:spaceId", r.handlers.AdminInstallationHandler.UpdateSpace)
	admin.GET("/activities", r.handlers.AdminInstallationHandler.ListActivities)
	admin.POST("/activities", r.handlers.AdminInstallationHandler.CreateActivity)
	admin.PATCH("/activities/:activityId", r.handlers.AdminInstallationHandler.UpdateActivity)
	admin.GET("/staff", r.handlers.AdminInstallationHandler.ListStaff)
	admin.PATCH("/users/:userId/role", r.handlers.AdminInstallationHandler.UpdateUserRole)
	admin.GET("/activities/:activityId/managers", r.handlers.AdminInstallationHandler.ListManagers)
	admin.PUT("/activities/:activityId/managers/:userId", r.handlers.AdminInstallationHandler.AssignManager)
	admin.DELETE("/activities/:activityId/managers/:userId", r.handlers.AdminInstallationHandler.RemoveManager)
}
