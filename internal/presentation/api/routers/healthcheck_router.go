package routers

func (r *Router) RegisterHealthcheckRoutes() {
	r.group.GET("/healthcheck", r.handlers.HealthcheckHandler.Get)
}
