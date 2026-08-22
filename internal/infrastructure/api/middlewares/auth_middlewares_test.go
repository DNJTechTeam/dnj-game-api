package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiCookies "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/auth"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/middlewares"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signedIdentityToken(t *testing.T, userID string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.IdentityClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "dnj-game-api", Audience: jwt.ClaimStrings{"dnj-v2"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	})
	signed, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return signed
}

func TestAuthenticationMiddleware_BearerAndTransportPrecedence(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_IDENTITY_SECRET", "test-secret")
	engine := gin.New()
	engine.GET("/protected", middlewares.AuthenticationMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"userId": common.ExtractUserIdFromContext(c.Request.Context())})
	})
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+signedIdentityToken(t, "41"))
	request.AddCookie(&http.Cookie{Name: apiCookies.IdentityTokenName, Value: signedIdentityToken(t, "99")})
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"userId":41}`, recorder.Body.String())
}

func TestAuthenticationMiddleware_RejectsNonHS256(t *testing.T) {
	// given
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_IDENTITY_SECRET", "test-secret")
	engine := gin.New()
	engine.GET("/protected", middlewares.AuthenticationMiddleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
	token := jwt.NewWithClaims(jwt.SigningMethodNone, auth.IdentityClaims{
		UserID: "1", RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "dnj-game-api", Audience: jwt.ClaimStrings{"dnj-v2"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	})
	raw, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", raw)
	recorder := httptest.NewRecorder()

	// when
	engine.ServeHTTP(recorder, request)

	// then
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
