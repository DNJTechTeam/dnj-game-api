package handlers

import (
	"net/http"

	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type EventSettingsHandler struct {
	EventSettingsService appInterfaces.EventSettingsServiceInterface
}

func (h *EventSettingsHandler) GetScoringStatus(c *gin.Context) {
	result, err := h.EventSettingsService.GetScoringStatus(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *EventSettingsHandler) CloseScoring(c *gin.Context) {
	request := &messages.CloseScoringRequestDTO{}
	if err := ParseStrictRequest(c, request); err != nil {
		request = &messages.CloseScoringRequestDTO{}
	}
	result, err := h.EventSettingsService.CloseScoring(c.Request.Context(), request.AuditNote)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *EventSettingsHandler) OpenScoring(c *gin.Context) {
	request := &messages.OpenScoringRequestDTO{}
	if err := ParseStrictRequest(c, request); err != nil {
		request = &messages.OpenScoringRequestDTO{}
	}
	result, err := h.EventSettingsService.OpenScoring(c.Request.Context(), request.AuditNote)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}
