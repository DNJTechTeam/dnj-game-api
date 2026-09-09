package routers

func (r *Router) RegisterEventSettingsRoutes() {
	admin := r.v2Group.Group("/admin/event-settings", r.authProtected()...)
	admin.GET("/scoring", r.handlers.EventSettingsHandler.GetScoringStatus)
	admin.POST("/scoring/close", r.handlers.EventSettingsHandler.CloseScoring)
	admin.POST("/scoring/open", r.handlers.EventSettingsHandler.OpenScoring)

	r.v2Group.GET("/event-settings/scoring", r.handlers.EventSettingsHandler.GetScoringStatus)
}
