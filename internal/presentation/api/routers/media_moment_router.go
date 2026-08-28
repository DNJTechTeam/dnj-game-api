package routers

func (r *Router) RegisterMediaMomentRoutes() {
	media := r.v2Group.Group("/media")
	media.POST(
		"/upload-intents",
		append(r.authProtected(), r.handlers.MediaHandler.CreateUploadIntent)...,
	)
	media.POST(
		"/:mediaAssetId/complete",
		append(r.authProtected(), r.handlers.MediaHandler.CompleteUpload)...,
	)

	moments := r.v2Group.Group("/moments")
	moments.GET("", append(r.authProtected(), r.handlers.MomentHandler.List)...)
	moments.POST("", append(r.authProtected(), r.handlers.MomentHandler.Create)...)
	moments.POST("/challenge", append(r.authProtected(), r.handlers.MomentHandler.CreateChallenge)...)
	moments.POST(
		"/:momentId/likes",
		append(r.authProtected(), r.handlers.MomentHandler.ToggleLike)...,
	)

	admin := r.v2Group.Group("/admin/moments")
	admin.GET(
		"/moderation",
		append(r.authProtected(), r.handlers.MomentHandler.ListModeration)...,
	)
	admin.POST(
		"/:momentId/moderation",
		append(r.authProtected(), r.handlers.MomentHandler.Moderate)...,
	)
}
