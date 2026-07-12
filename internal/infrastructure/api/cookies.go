package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
)

const (
	// IdentityTokenName is the cookie that carries the identity JWT. The same
	// token may also be sent in the Authorization header (see AuthenticationMiddleware).
	IdentityTokenName = "identity_token"
	cookiePath        = "/"
	cookieHttpOnly    = true
)

func GetCookieSecure() bool {
	return !common.EnvironmentIs(common.EnvironmentLocalhost)
}

func getCookieDomain() string {
	if domain, ok := os.LookupEnv("COOKIE_DOMAIN"); ok && domain != "" {
		return domain
	}
	return ""
}

// getCookieSameSite returns SameSite=Lax for localhost and SameSite=None for
// all deployed environments (cross-origin SPA + API). SameSite=None requires Secure=true.
func getCookieSameSite() http.SameSite {
	if common.EnvironmentIs(common.EnvironmentLocalhost) {
		return http.SameSiteLaxMode
	}
	return http.SameSiteNoneMode
}

func SetIdentityToken(c *gin.Context, token string) {
	secure := GetCookieSecure()
	c.SetSameSite(getCookieSameSite())
	c.SetCookie(IdentityTokenName, token, 0, cookiePath, getCookieDomain(), secure, cookieHttpOnly)
}

func Logout(c *gin.Context) {
	secure := GetCookieSecure()
	c.SetSameSite(getCookieSameSite())
	c.SetCookie(IdentityTokenName, "", -1, cookiePath, getCookieDomain(), secure, cookieHttpOnly)
}
