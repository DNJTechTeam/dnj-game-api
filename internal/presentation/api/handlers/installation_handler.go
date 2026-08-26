package handlers

import (
	"context"
	"net/http"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type InstallationHandler struct {
	SpaceService    interfaces.SpaceServiceInterface
	ActivityService interfaces.ActivityServiceInterface
}

func (h *InstallationHandler) ListSpaces(c *gin.Context) {
	filter := &messages.ListSpacesFilterDTO{}
	ParsePaginationFromQuery(c, filter)
	result, err := h.SpaceService.List(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	setPaginationHeaders(c, result.Pagination)
	ResponseSuccess(c, http.StatusOK, result.Data)
}

func (h *InstallationHandler) StartActivity(c *gin.Context) {
	h.transitionActivity(c, h.ActivityService.Start)
}

func (h *InstallationHandler) PauseActivity(c *gin.Context) {
	h.transitionActivity(c, h.ActivityService.Pause)
}

func (h *InstallationHandler) ConcludeActivity(c *gin.Context) {
	h.transitionActivity(c, h.ActivityService.Conclude)
}

func (h *InstallationHandler) transitionActivity(c *gin.Context, operation func(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error)) {
	result, err := operation(c.Request.Context(), c.Param("id"), c.GetHeader("Idempotency-Key"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}
