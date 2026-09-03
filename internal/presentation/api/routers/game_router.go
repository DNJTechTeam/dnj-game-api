package routers

func (r *Router) RegisterGameRoutes() {
	r.v2Group.GET("/games", r.handlers.GameHandler.List)
	r.v2Group.GET("/games/:gameId", r.handlers.GameHandler.Get)
	r.v2Group.GET("/rankings", r.handlers.GameHandler.Rankings)
	r.v2Group.GET("/game/overview", append(r.authProtected(), r.handlers.GameHandler.Overview)...)
	r.v2Group.GET("/activity-runs/current", append(r.authProtected(), r.handlers.GameHandler.CurrentRun)...)
	r.v2Group.GET("/participations/current", append(r.authProtected(), r.handlers.GameHandler.CurrentParticipation)...)
	r.v2Group.POST("/qr/validate", append(r.authProtected(), r.handlers.GameHandler.ValidateQR)...)
	r.v2Group.GET("/admin/activities/:activityId/qr", append(r.authProtected(), r.handlers.GameHandler.AdminCheckpointQR)...)

	manager := r.v2Group.Group("/manager")
	manager.GET("/game-overview", append(r.authProtected(), r.handlers.GameHandler.ManagerOverview)...)
	manager.POST("/games", append(r.authProtected(), r.handlers.GameHandler.CreateManagerGame)...)
	manager.PATCH("/games/:gameId", append(r.authProtected(), r.handlers.GameHandler.UpdateManagerGame)...)
	manager.GET("/runs/:runId", append(r.authProtected(), r.handlers.GameHandler.ManagerRun)...)
	manager.POST("/runs", append(r.authProtected(), r.handlers.GameHandler.CreateRun)...)
	manager.POST("/runs/:runId/qr", append(r.authProtected(), r.handlers.GameHandler.RotateQR)...)
	manager.POST("/runs/:runId/start", append(r.authProtected(), r.handlers.GameHandler.StartRun)...)
	manager.POST("/runs/:runId/pause", append(r.authProtected(), r.handlers.GameHandler.PauseRun)...)
	manager.POST("/runs/:runId/resume", append(r.authProtected(), r.handlers.GameHandler.ResumeRun)...)
	manager.POST("/runs/:runId/results", append(r.authProtected(), r.handlers.GameHandler.FinalizeRun)...)
	manager.POST("/runs/:runId/cancel", append(r.authProtected(), r.handlers.GameHandler.CancelRun)...)
}
