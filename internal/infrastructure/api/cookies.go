package api

import (
	"net/http"
	"os"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/gin-gonic/gin"
)

const (
	// IdentityTokenName is the cookie that carries the identity JWT. The same
	// token may also be sent in the Authorization header (see AuthenticationMiddleware).
	IdentityTokenName = "identity_token"
	RefreshTokenName  = "refresh_token"
	CSRFTokenName     = "csrf_token"
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

func SetIdentitySession(c *gin.Context, accessToken, refreshToken, csrfToken string) {
	secure := GetCookieSecure()
	c.SetSameSite(getCookieSameSite())
	c.SetCookie(IdentityTokenName, accessToken, 15*60, cookiePath, getCookieDomain(), secure, true)
	c.SetCookie(RefreshTokenName, refreshToken, 30*24*60*60, "/v2/auth", getCookieDomain(), secure, true)
	c.SetCookie(CSRFTokenName, csrfToken, 30*24*60*60, "/v2/auth", getCookieDomain(), secure, false)
}

func Logout(c *gin.Context) {
	secure := GetCookieSecure()
	c.SetSameSite(getCookieSameSite())
	c.SetCookie(IdentityTokenName, "", -1, cookiePath, getCookieDomain(), secure, cookieHttpOnly)
	c.SetCookie(RefreshTokenName, "", -1, "/v2/auth", getCookieDomain(), secure, true)
	c.SetCookie(CSRFTokenName, "", -1, "/v2/auth", getCookieDomain(), secure, false)
}
