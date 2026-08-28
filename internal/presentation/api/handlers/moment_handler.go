package handlers

import (
	"net/http"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type MomentHandler struct {
	MomentService interfaces.MomentServiceInterface
}

func (h *MomentHandler) List(c *gin.Context) {
	if !validatePublishedQuery(c, "scope", "cursor") {
		return
	}
	scope, present := c.GetQuery("scope")
	if !present {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "scope é obrigatório.", nil)
		return
	}
	response, err := h.MomentService.List(c.Request.Context(), scope, c.Query("cursor"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *MomentHandler) Create(c *gin.Context) {
	if !validatePublishedQuery(c) {
		return
	}
	var request messages.CreateMomentRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Envie somente mediaAssetId, publishConsent e participationId opcional.",
			nil,
		)
		return
	}
	response, status, err := h.MomentService.Create(
		c.Request.Context(),
		c.GetHeader("Idempotency-Key"),
		&request,
	)
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, status, response)
}

func (h *MomentHandler) CreateChallenge(c *gin.Context) {
	if !validatePublishedQuery(c) {
		return
	}
	var request messages.CreateChallengeMomentRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Envie somente mediaAssetId e publishConsent.",
			nil,
		)
		return
	}
	response, status, err := h.MomentService.Create(
		c.Request.Context(),
		c.GetHeader("Idempotency-Key"),
		&messages.CreateMomentRequestDTO{
			MediaAssetID:   request.MediaAssetID,
			PublishConsent: request.PublishConsent,
			ChallengeMode:  true,
		},
	)
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, status, response)
}

func (h *MomentHandler) ToggleLike(c *gin.Context) {
	if !validatePublishedQuery(c) || !requireEmptyBody(c) {
		return
	}
	response, err := h.MomentService.ToggleLike(
		c.Request.Context(),
		c.Param("momentId"),
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *MomentHandler) ListModeration(c *gin.Context) {
	if !validatePublishedQuery(c, "queue", "page") {
		return
	}
	queue, present := c.GetQuery("queue")
	if !present {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "queue é obrigatória.", nil)
		return
	}
	page, ok := parsePublicPage(c)
	if !ok {
		return
	}
	response, err := h.MomentService.ListModeration(c.Request.Context(), queue, page)
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *MomentHandler) Moderate(c *gin.Context) {
	if !validatePublishedQuery(c) {
		return
	}
	var request messages.ModerationRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente action.", nil)
		return
	}
	response, err := h.MomentService.Moderate(
		c.Request.Context(),
		c.Param("momentId"),
		c.GetHeader("Idempotency-Key"),
		&request,
	)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}
