package middlewares

import (
	"github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"

	"github.com/gin-gonic/gin"
)

// WebhookSecretMiddleware protects public webhook endpoints with a shared
// secret sent by the external platform in the X-Webhook-Secret header,
// since these routes cannot require a user JWT.
func WebhookSecretMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := common.GetEnv("SUBSCRIPTION_WEBHOOK_SECRET")
		got := c.GetHeader("X-Webhook-Secret")

		if got == "" || got != expected {
			handlers.ResponseUnauthorized(c, errors.NewSimpleError("Webhook secret inválido."))
			c.Abort()
			return
		}

		c.Next()
	}
}
