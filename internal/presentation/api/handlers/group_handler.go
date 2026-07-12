package handlers

import (
	"net/http"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"

	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	GroupService interfaces.GroupServiceInterface
}

// Search godoc
// @Summary      Busca grupos por nome
// @Description  Busca case-insensitive por nome (mínimo 3 caracteres), limitada a 20 resultados.
// @Tags         groups
// @Produce      json
// @Param        search query string true "Termo de busca (mínimo 3 caracteres)"
// @Success      200 {array} messages.GroupSummaryDTO
// @Failure      400 {object} appErrors.Error
// @Security     BearerAuth
// @Router       /groups [get]
func (h *GroupHandler) Search(c *gin.Context) {
	var response []*messages.GroupSummaryDTO
	response, err := h.GroupService.Search(c.Request.Context(), c.Query("search"))
	if err != nil {
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}
