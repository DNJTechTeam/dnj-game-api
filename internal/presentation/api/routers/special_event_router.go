package routers

func (r *Router) RegisterSpecialEventRoutes() {
	if r.handlers.SpecialEventHandler == nil {
		return
	}
	manager := r.v2Group.Group("/manager/special-events", r.authProtected()...)
	manager.GET("", r.handlers.SpecialEventHandler.ListManager)
	manager.POST("", r.handlers.SpecialEventHandler.Create)
	manager.POST("/teaser", r.handlers.SpecialEventHandler.Teaser)
	manager.POST("/qr", r.handlers.SpecialEventHandler.QR)
	manager.POST("/close", r.handlers.SpecialEventHandler.Close)
	r.v2Group.GET("/special-events/active", append(r.authProtected(), r.handlers.SpecialEventHandler.Active)...)
	r.v2Group.GET("/live-display", r.handlers.SpecialEventHandler.Display)
}
