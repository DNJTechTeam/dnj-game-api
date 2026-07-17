package routers

func (r *Router) RegisterSubscriptionRoutes() {
	group := r.group.Group("/subscriptions")

	group.POST("/webhook", r.handlers.SubscriptionWebhookHandler.Ingest)
}
