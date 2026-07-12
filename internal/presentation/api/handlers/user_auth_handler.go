package handlers

import (
	"net/http"

	"github.com/scienceandcode/nucleus-api/internal/app/errors"
	"github.com/scienceandcode/nucleus-api/internal/app/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/app/messages"
	apiHelpers "github.com/scienceandcode/nucleus-api/internal/infrastructure/api"

	"github.com/gin-gonic/gin"
)

type UserAuthHandler struct {
	UserAuthService interfaces.UserAuthServiceInterface
}

func (h *UserAuthHandler) Register(c *gin.Context) {
	var request messages.RegisterRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, errors.NewError(err.Error(), nil))
		return
	}

	response, err := h.UserAuthService.Register(c.Request.Context(), &request)
	if err != nil {
		ResponseBadRequest(c, err.(*errors.Error))
		return
	}

	ResponseSuccess(c, http.StatusCreated, response)
}

func (h *UserAuthHandler) Login(c *gin.Context) {
	var request messages.LoginRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, errors.NewError(err.Error(), nil))
		return
	}

	response, token, err := h.UserAuthService.Login(c.Request.Context(), &request)
	if err != nil {
		ResponseUnauthorized(c, err)
		return
	}

	// The identity token is returned both as an HttpOnly cookie (browser SPAs)
	// and is equally usable as an Authorization header (mobile / server clients).
	apiHelpers.SetIdentityToken(c, token)

	ResponseSuccess(c, http.StatusOK, response)
}

func (h *UserAuthHandler) ConfirmEmail(c *gin.Context) {
	var request messages.ConfirmEmailRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, errors.NewError(err.Error(), nil))
		return
	}

	response, err := h.UserAuthService.ConfirmEmail(c.Request.Context(), request.Token)
	if err != nil {
		ResponseBadRequest(c, err.(*errors.Error))
		return
	}

	apiHelpers.Logout(c)

	ResponseSuccess(c, http.StatusOK, response)
}

func (h *UserAuthHandler) ResendConfirmationEmail(c *gin.Context) {
	response, err := h.UserAuthService.ResendConfirmationEmail(c.Request.Context())
	if err != nil {
		ResponseBadRequest(c, err.(*errors.Error))
		return
	}

	ResponseSuccess(c, http.StatusOK, response)
}

func (h *UserAuthHandler) UpdatePassword(c *gin.Context) {
	var request messages.UpdatePasswordRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, errors.NewError(err.Error(), nil))
		return
	}

	response, err := h.UserAuthService.UpdatePassword(c.Request.Context(), &request)
	if err != nil {
		ResponseBadRequest(c, err.(*errors.Error))
		return
	}

	apiHelpers.Logout(c)

	ResponseSuccess(c, http.StatusOK, response)
}

func (h *UserAuthHandler) ForgotPassword(c *gin.Context) {
	var request messages.ForgotPasswordRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, errors.NewError(err.Error(), nil))
		return
	}

	if err := h.UserAuthService.ForgotPassword(c.Request.Context(), &request); err != nil {
		ResponseBadRequest(c, err.(*errors.Error))
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UserAuthHandler) Logout(c *gin.Context) {
	apiHelpers.Logout(c)
	c.Status(http.StatusNoContent)
}
