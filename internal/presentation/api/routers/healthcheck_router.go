package routers

func (r *Router) RegisterHealthcheckRoutes() {
	r.group.GET("/healthcheck", r.handlers.HealthcheckHandler.Get)
	r.v2Group.GET("/healthcheck", r.handlers.HealthcheckHandler.GetV2)
	r.v2Group.GET("/readiness", r.handlers.HealthcheckHandler.Ready)
}
