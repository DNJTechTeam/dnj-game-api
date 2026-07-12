package middlewares

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	apiCookies "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/auth"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
)

// AuthMiddlewareCustomizer lets specific routes relax the standard checks.
// For example, /auth/confirm-email must be reachable before the email is
// confirmed, and /auth/change-password before the password is set.
type AuthMiddlewareCustomizer struct {
	SkipPasswordUpdatedValidation bool
	SkipEmailConfirmedValidation  bool
}

// AuthenticationMiddleware validates the identity JWT and puts the user id in
// the request context under the "userId" key. The token is read from the
// Authorization header first and falls back to the identity_token cookie.
func AuthenticationMiddleware(customizer *AuthMiddlewareCustomizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")

		fromCookie, _ := c.Cookie(apiCookies.IdentityTokenName)
		if fromCookie != "" {
			token = fromCookie
		}

		if token == "" {
			handlers.ResponseUnauthorized(c, errors.NewSimpleError("Token is required."))
			c.Abort()
			return
		}

		identityClaims := &auth.IdentityClaims{}
		claims, err := jwt.ParseWithClaims(token, identityClaims, func(token *jwt.Token) (interface{}, error) {
			return []byte(common.GetEnv("JWT_IDENTITY_SECRET")), nil
		})

		if err != nil {
			handlers.ResponseUnauthorized(c, errors.NewSimpleError("Token is invalid."))
			c.Abort()
			return
		}

		if !claims.Valid {
			handlers.ResponseUnauthorized(c, errors.NewSimpleError("Token is invalid."))
			c.Abort()
			return
		}

		if time.Now().After(identityClaims.ExpiresAt.Time) {
			handlers.ResponseUnauthorized(c, errors.NewSimpleError("Token is expired."))
			c.Abort()
			return
		}

		if customizer == nil {
			customizer = &AuthMiddlewareCustomizer{}
		}

		if !customizer.SkipPasswordUpdatedValidation && !identityClaims.HasUpdatedPassword {
			handlers.ResponseUnauthorized(c, errors.NewSimpleError("A senha deve ser atualizada para entrar no sistema."))
			c.Abort()
			return
		}

		if !customizer.SkipEmailConfirmedValidation && !identityClaims.EmailConfirmed {
			handlers.ResponseUnauthorized(c, errors.NewSimpleError("Email deve ser confirmado para acessar este recurso."))
			c.Abort()
			return
		}

		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), common.UserIDContextKey, identityClaims.UserID))
		c.Next()
	}
}
