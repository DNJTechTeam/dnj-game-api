package handlers

import (
	"net/http"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	ProfileService interfaces.ProfileServiceInterface
}

func (h *ProfileHandler) Current(c *gin.Context) {
	result, err := h.ProfileService.Current(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *ProfileHandler) Update(c *gin.Context) {
	var request messages.UpdateCurrentProfileRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Somente name, mobilePhone e avatarUrl podem ser editados.", nil)
		return
	}
	result, err := h.ProfileService.Update(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}
