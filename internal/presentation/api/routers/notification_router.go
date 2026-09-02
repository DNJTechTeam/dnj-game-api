package routers

func (r *Router) RegisterNotificationRoutes() {
	notifications := r.v2Group.Group("/notifications")
	notifications.GET("/preferences", append(r.authProtected(), r.handlers.NotificationHandler.GetPreferences)...)
	notifications.PUT("/preferences", append(r.authProtected(), r.handlers.NotificationHandler.UpdatePreferences)...)
	notifications.GET("", append(r.authProtected(), r.handlers.NotificationHandler.List)...)
	notifications.POST("/:notificationId/read", append(r.authProtected(), r.handlers.NotificationHandler.MarkRead)...)
	push := r.v2Group.Group("/push")
	push.GET("/config", append(r.authProtected(), r.handlers.NotificationHandler.GetPushConfig)...)
	push.PUT("/subscriptions", append(r.authProtected(), r.handlers.NotificationHandler.UpsertPushSubscription)...)
	push.DELETE("/subscriptions", append(r.authProtected(), r.handlers.NotificationHandler.DeactivatePushSubscription)...)
	r.v2Group.POST("/internal/notifications/queue-called", r.handlers.NotificationHandler.QueueCalled)

	admin := r.v2Group.Group("/admin/notifications")
	admin.POST("", append(r.authProtected(), r.handlers.NotificationHandler.AdminSend)...)
}
