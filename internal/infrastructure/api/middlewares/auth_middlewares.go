package middlewares

import (
	"context"
	"net/http"
	"strings"

	apiCookies "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/auth"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthenticationMiddleware validates the identity JWT and puts the user id in
// the request context under the "userId" key. The token is read from the
// Authorization header first and falls back to the identity_token cookie.
func AuthenticationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")

		if token == "" {
			fromCookie, _ := c.Cookie(apiCookies.IdentityTokenName)
			token = fromCookie
		}
		token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))

		if token == "" {
			handlers.ResponseAPIError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.", nil)
			return
		}

		identityClaims := &auth.IdentityClaims{}
		claims, err := jwt.ParseWithClaims(token, identityClaims, func(token *jwt.Token) (interface{}, error) {
			return []byte(common.GetEnv("JWT_IDENTITY_SECRET")), nil
		},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer("dnj-game-api"),
			jwt.WithAudience("dnj-v2"),
			jwt.WithExpirationRequired(),
		)

		if err != nil {
			handlers.ResponseAPIError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.", nil)
			return
		}

		if !claims.Valid {
			handlers.ResponseAPIError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.", nil)
			return
		}

		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), common.UserIDContextKey, identityClaims.UserID))
		c.Next()
	}
}
