package routers

import "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/middlewares"

func (r *Router) RegisterSubscriptionRoutes() {
	group := r.group.Group("/subscriptions")

	group.POST("/webhook", middlewares.WebhookSecretMiddleware(), r.handlers.SubscriptionWebhookHandler.Ingest)
}
