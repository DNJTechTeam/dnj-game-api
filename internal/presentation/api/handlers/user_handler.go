package handlers

import (
	"errors"
	"net/http"
	"strconv"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	UserService interfaces.UserServiceInterface
}

// UpdateGroup godoc
// @Summary      Atualiza o grupo de um usuário
// @Description  Vincula o usuário a um grupo existente (groupId) ou cria um novo grupo pelo nome (groupName) e vincula.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id path uint64 true "ID do usuário"
// @Param        request body messages.UpdateUserGroupRequestDTO true "Grupo a vincular"
// @Success      200 {object} messages.UserResponseDTO
// @Failure      400 {object} appErrors.Error
// @Failure      404 {object} appErrors.Error
// @Security     BearerAuth
// @Router       /users/{id}/update-group [post]
func (h *UserHandler) UpdateGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ResponseNotFound(c, appErrors.ErrNotFound)
		return
	}

	var request messages.UpdateUserGroupRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, appErrors.NewError(err.Error(), nil))
		return
	}

	response, err := h.UserService.UpdateGroup(c.Request.Context(), id, &request)
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			ResponseNotFound(c, appErrors.ErrNotFound)
			return
		}
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}
