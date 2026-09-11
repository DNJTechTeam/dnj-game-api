package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	apiCookies "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api"
	"github.com/gin-gonic/gin"
)

type IdentityHandler struct {
	IdentityService interfaces.IdentityServiceInterface
}

func identityFailure(c *gin.Context, err error) {
	if apiErr, ok := err.(*appErrors.APIServiceError); ok {
		ResponseAPIError(c, apiErr.Status, apiErr.Code, apiErr.Message, apiErr.Details)
		return
	}
	ResponseAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Ocorreu um erro interno.", nil)
}

func csrfValid(c *gin.Context) bool {
	cookie, err := c.Cookie(apiCookies.CSRFTokenName)
	header := c.GetHeader("X-CSRF-Token")
	return err == nil && cookie != "" && len(cookie) == len(header) && subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) == 1
}

func setIdentityResponse(c *gin.Context, response *messages.IdentitySessionResponseDTO) {
	apiCookies.SetIdentitySession(c, response.AccessToken, response.RefreshToken, response.CSRFToken)
}

// refreshTokenFromRequest resolves the refresh token for /auth/refresh and
// /auth/logout. Bearer clients send it as JSON ({"refreshToken": "..."}) and
// skip cookies and CSRF entirely. Legacy cookie clients keep the double-submit
// CSRF check. When ok is false a response has already been written.
func refreshTokenFromRequest(c *gin.Context) (token string, fromCookie bool, ok bool) {
	if c.Request.ContentLength != 0 && strings.HasPrefix(c.ContentType(), "application/json") {
		var body messages.RefreshTokenRequestDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Corpo da requisição inválido.", nil)
			return "", false, false
		}
		if token = strings.TrimSpace(body.RefreshToken); token != "" {
			return token, false, true
		}
	}
	cookie, err := c.Cookie(apiCookies.RefreshTokenName)
	if err != nil || cookie == "" {
		return "", false, true
	}
	if !csrfValid(c) {
		ResponseAPIError(c, http.StatusForbidden, "CSRF_INVALID", "Token CSRF inválido.", nil)
		return "", true, false
	}
	return cookie, true, true
}

func (h *IdentityHandler) Google(c *gin.Context) {
	var request messages.GoogleAuthRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "idToken é obrigatório.", nil)
		return
	}
	response, err := h.IdentityService.AuthenticateGoogle(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	setIdentityResponse(c, response)
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *IdentityHandler) SignupWithEmail(c *gin.Context) {
	var request messages.EmailSignupRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Email é obrigatório.", nil)
		return
	}
	response, err := h.IdentityService.SignupWithEmail(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *IdentityHandler) VerifyEmailSignup(c *gin.Context) {
	var request messages.VerifyEmailSignupRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Email e código são obrigatórios.", nil)
		return
	}
	response, err := h.IdentityService.VerifyEmailSignup(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	setIdentityResponse(c, response)
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *IdentityHandler) Refresh(c *gin.Context) {
	refreshToken, fromCookie, ok := refreshTokenFromRequest(c)
	if !ok {
		return
	}
	response, err := h.IdentityService.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		if fromCookie {
			apiCookies.Logout(c)
		}
		identityFailure(c, err)
		return
	}
	if fromCookie {
		setIdentityResponse(c, response)
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *IdentityHandler) Current(c *gin.Context) {
	response, err := h.IdentityService.Current(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *IdentityHandler) CompleteOnboarding(c *gin.Context) {
	var request messages.CompleteOnboardingRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "CPF, telefone e grupo são obrigatórios.", nil)
		return
	}
	response, err := h.IdentityService.CompleteOnboarding(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *IdentityHandler) Logout(c *gin.Context) {
	refreshToken, fromCookie, ok := refreshTokenFromRequest(c)
	if !ok {
		return
	}
	if err := h.IdentityService.Logout(c.Request.Context(), refreshToken); err != nil {
		identityFailure(c, err)
		return
	}
	if fromCookie {
		apiCookies.Logout(c)
	}
	ResponseSuccess(c, http.StatusOK, messages.LogoutResponseDTO{Status: "logged_out"})
}
