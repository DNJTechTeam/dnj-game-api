package handlers

import (
	"crypto/subtle"
	"net/http"

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
	if !csrfValid(c) {
		ResponseAPIError(c, http.StatusForbidden, "CSRF_INVALID", "Token CSRF inválido.", nil)
		return
	}
	refreshToken, _ := c.Cookie(apiCookies.RefreshTokenName)
	response, err := h.IdentityService.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		apiCookies.Logout(c)
		identityFailure(c, err)
		return
	}
	setIdentityResponse(c, response)
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
	if !csrfValid(c) {
		ResponseAPIError(c, http.StatusForbidden, "CSRF_INVALID", "Token CSRF inválido.", nil)
		return
	}
	refreshToken, _ := c.Cookie(apiCookies.RefreshTokenName)
	if err := h.IdentityService.Logout(c.Request.Context(), refreshToken); err != nil {
		identityFailure(c, err)
		return
	}
	apiCookies.Logout(c)
	ResponseSuccess(c, http.StatusOK, messages.LogoutResponseDTO{Status: "logged_out"})
}
