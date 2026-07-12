package handlers

import (
	"net/http"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	apiHelpers "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api"

	"github.com/gin-gonic/gin"
)

// AuthHandler exposes the passwordless onboarding flow: a subscriber
// confirms email+document, receives a 6-digit code by email, and exchanges
// it for an identity token.
type AuthHandler struct {
	AuthService interfaces.AuthServiceInterface
}

// Onboarding godoc
// @Summary      Confirma inscrição e envia código de verificação
// @Description  Verifica se o email+documento existem na base de inscrições recebida via webhook e envia um código de 6 dígitos por email.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body messages.OnboardingRequestDTO true "Email e documento do inscrito"
// @Success      204 "Código enviado"
// @Failure      400 {object} appErrors.Error
// @Router       /auth/onboarding [post]
func (h *AuthHandler) Onboarding(c *gin.Context) {
	var request messages.OnboardingRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, appErrors.NewError(err.Error(), nil))
		return
	}

	if err := h.AuthService.Onboarding(c.Request.Context(), &request); err != nil {
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	c.Status(http.StatusNoContent)
}

// VerifyCode godoc
// @Summary      Verifica o código e retorna o token de identidade
// @Description  Confirma o código de verificação, cria o usuário na primeira verificação (idempotente nas seguintes) e retorna o identityToken.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body messages.VerificationCodeRequestDTO true "Email e código de verificação"
// @Success      200 {object} messages.VerificationCodeResponseDTO
// @Failure      400 {object} appErrors.Error
// @Router       /auth/verification-code [post]
func (h *AuthHandler) VerifyCode(c *gin.Context) {
	var request messages.VerificationCodeRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, appErrors.NewError(err.Error(), nil))
		return
	}

	response, err := h.AuthService.VerifyCode(c.Request.Context(), &request)
	if err != nil {
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}

	apiHelpers.SetIdentityToken(c, response.IdentityToken)
	ResponseSuccess(c, http.StatusOK, response)
}
