package handlers

import (
	"net/http"
	"strconv"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type GroupInviteHandler struct {
	GroupInviteService interfaces.GroupInviteServiceInterface
}

func parsePositiveID(c *gin.Context, name string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || value == 0 {
		ResponseAPIError(c, http.StatusNotFound, "NOT_FOUND", "Recurso não encontrado.", nil)
		return 0, false
	}
	return value, true
}

func (h *GroupInviteHandler) Create(c *gin.Context) {
	groupID, ok := parsePositiveID(c, "groupId")
	if !ok {
		return
	}
	result, err := h.GroupInviteService.Create(c.Request.Context(), groupID)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusCreated, result)
}

func (h *GroupInviteHandler) Renew(c *gin.Context) {
	groupID, ok := parsePositiveID(c, "groupId")
	if !ok {
		return
	}
	inviteID, ok := parsePositiveID(c, "inviteId")
	if !ok {
		return
	}
	result, err := h.GroupInviteService.Renew(c.Request.Context(), groupID, inviteID)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusCreated, result)
}

func (h *GroupInviteHandler) Revoke(c *gin.Context) {
	groupID, ok := parsePositiveID(c, "groupId")
	if !ok {
		return
	}
	inviteID, ok := parsePositiveID(c, "inviteId")
	if !ok {
		return
	}
	if err := h.GroupInviteService.Revoke(c.Request.Context(), groupID, inviteID); err != nil {
		identityFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *GroupInviteHandler) List(c *gin.Context) {
	groupID, ok := parsePositiveID(c, "groupId")
	if !ok {
		return
	}
	filter := &messages.ListGroupInvitesFilterDTO{}
	ParsePaginationFromQuery(c, filter)
	result, err := h.GroupInviteService.List(c.Request.Context(), groupID, filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *GroupInviteHandler) Consume(c *gin.Context) {
	var request messages.ConsumeGroupInviteRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "code é obrigatório.", nil)
		return
	}
	result, err := h.GroupInviteService.Consume(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}
