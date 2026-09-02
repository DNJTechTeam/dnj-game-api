package handlers

import (
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
	"net/http"
)

type SpecialEventHandler struct {
	Service appInterfaces.SpecialEventServiceInterface
}

func (h *SpecialEventHandler) ListManager(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	out, err := h.Service.ListManager(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, out)
}
func (h *SpecialEventHandler) Create(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var req messages.CreateSpecialEventRequestDTO
	if err := ParseStrictRequest(c, &req); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie os dados do evento especial.", nil)
		return
	}
	out, err := h.Service.Create(c.Request.Context(), &req)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusCreated, out)
}
func (h *SpecialEventHandler) Teaser(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var req messages.SpecialEventIDRequestDTO
	if err := ParseStrictRequest(c, &req); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente eventId.", nil)
		return
	}
	out, err := h.Service.Teaser(c.Request.Context(), req.EventID)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, out)
}
func (h *SpecialEventHandler) QR(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var req messages.SpecialEventIDRequestDTO
	if err := ParseStrictRequest(c, &req); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente eventId.", nil)
		return
	}
	out, err := h.Service.ReleaseQR(c.Request.Context(), req.EventID)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, out)
}
func (h *SpecialEventHandler) Close(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var req messages.SpecialEventIDRequestDTO
	if err := ParseStrictRequest(c, &req); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente eventId.", nil)
		return
	}
	if err := h.Service.Close(c.Request.Context(), req.EventID); err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, gin.H{"ok": true})
}
func (h *SpecialEventHandler) Active(c *gin.Context) {
	if !requirePublishedQuery(c, "target") {
		return
	}
	out, err := h.Service.Active(c.Request.Context(), c.Query("target"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, out)
}
func (h *SpecialEventHandler) Display(c *gin.Context) {
	if !requirePublishedQuery(c, "target") {
		return
	}
	out, err := h.Service.Display(c.Request.Context(), c.Query("target"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, out)
}
