package handlers

import (
	"context"
	"net/http"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type FavoriteHandler struct {
	FavoriteService interfaces.FavoriteServiceInterface
}

func (h *FavoriteHandler) List(c *gin.Context) {
	if !validatePublishedQuery(c, "page") {
		return
	}
	page, ok := parsePublicPage(c)
	if !ok {
		return
	}
	filter := &messages.ListFavoritesFilterDTO{}
	filter.SetPage(page)
	result, err := h.FavoriteService.List(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *FavoriteHandler) Put(c *gin.Context) {
	h.write(c, h.FavoriteService.Put)
}

func (h *FavoriteHandler) Delete(c *gin.Context) {
	h.write(c, h.FavoriteService.Delete)
}

func (h *FavoriteHandler) write(c *gin.Context, operation func(ctx context.Context, activityID, idempotencyKey string) error) {
	if !validatePublishedQuery(c) {
		return
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || c.Request.ContentLength != 0 {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Uma Idempotency-Key UUID e corpo vazio são obrigatórios.", nil)
		return
	}
	if err := operation(c.Request.Context(), c.Param("activityId"), keys[0]); err != nil {
		identityFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
