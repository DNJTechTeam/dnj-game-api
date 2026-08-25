package handlers

import (
	"net/http"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type MediaHandler struct {
	MediaService interfaces.MediaServiceInterface
}

func (h *MediaHandler) CreateUploadIntent(c *gin.Context) {
	if !validatePublishedQuery(c) {
		return
	}
	var request messages.CreateUploadIntentRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Envie somente contentType, bytes e checksumSha256.",
			nil,
		)
		return
	}
	response, status, err := h.MediaService.CreateUploadIntent(
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

func (h *MediaHandler) CompleteUpload(c *gin.Context) {
	if !validatePublishedQuery(c) || !requireEmptyBody(c) {
		return
	}
	response, status, err := h.MediaService.CompleteUpload(
		c.Request.Context(),
		c.Param("mediaAssetId"),
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, status, response)
}
