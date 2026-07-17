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
// confirms their document (CPF), receives a 6-digit code by email once an
// email is on file, and exchanges the code for an identity token.
type AuthHandler struct {
	AuthService interfaces.AuthServiceInterface
}

// Onboarding godoc
// @Summary      Confirma inscrição e envia código de verificação
// @Description  Localiza a inscrição pelo documento (CPF). Se o registro já tem email, envia o código e retorna status CODE_SENT com o email ofuscado. Se não tem email, retorna status EMAIL_REQUIRED sem enviar nada — a mesma rota deve ser chamada de novo com o campo email preenchido para completar o cadastro e enviar o código.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body messages.OnboardingRequestDTO true "Documento do inscrito (e opcionalmente o email, quando ainda não cadastrado)"
// @Success      200 {object} messages.OnboardingResponseDTO
// @Failure      400 {object} appErrors.Error
// @Router       /auth/onboarding [post]
func (h *AuthHandler) Onboarding(c *gin.Context) {
	var request messages.OnboardingRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, appErrors.NewError(err.Error(), nil))
		return
	}

	response, err := h.AuthService.Onboarding(c.Request.Context(), &request)
	if err != nil {
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
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
