package handlers

import (
	"net/http"
	"strconv"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	GroupService interfaces.GroupServiceInterface
}

func setPaginationHeaders(c *gin.Context, pagination messages.Pagination) {
	c.Header("X-Page", pagination.CurrentPage.String())
	c.Header("X-Limit", strconv.Itoa(pagination.Limit))
	c.Header("X-Has-Next-Page", strconv.FormatBool(pagination.HasNextPage))
}

// Search godoc
// @Summary      Busca grupos por nome
// @Description  Lista grupos ou busca case-insensitive por nome, limitada a 20 resultados por página.
// @Tags         groups
// @Produce      json
// @Param        search query string false "Termo de busca (vazio ou mínimo 3 caracteres)"
// @Param        page query int false "Página 1-indexed"
// @Success      200 {array} messages.GroupSummaryDTO
// @Failure      400 {object} APIError
// @Security     BearerAuth
// @Router       /groups [get]
func (h *GroupHandler) Search(c *gin.Context) {
	filter := &messages.ListGroupsFilterDTO{}
	ParsePaginationFromQuery(c, filter)
	result, err := h.GroupService.Search(c.Request.Context(), c.Query("search"), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	setPaginationHeaders(c, result.Pagination)
	ResponseSuccess(c, http.StatusOK, result.Data)
}

func (h *GroupHandler) Current(c *gin.Context) {
	result, err := h.GroupService.Current(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *GroupHandler) UpdateCurrent(c *gin.Context) {
	var request messages.UpdateCurrentGroupRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "groupId deve ser um identificador válido ou null.", nil)
		return
	}
	result, err := h.GroupService.UpdateCurrent(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *GroupHandler) Members(c *gin.Context) {
	filter := &messages.ListGroupMembersFilterDTO{}
	ParsePaginationFromQuery(c, filter)
	result, err := h.GroupService.Members(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}
