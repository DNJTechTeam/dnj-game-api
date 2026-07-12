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

// TaskHandler is the reference HTTP handler. Fields are populated by Wire
// (wire.Struct with "*"), so any new dependency just needs to be a field here.
type TaskHandler struct {
	TaskService interfaces.TaskServiceInterface
}

// List godoc
// @Summary      Lista as tarefas do usuário autenticado
// @Tags         tasks
// @Produce      json
// @Success      200 {array} messages.TaskResponseDTO
// @Failure      400 {object} appErrors.Error
// @Security     BearerAuth
// @Router       /tasks [get]
func (h *TaskHandler) List(c *gin.Context) {
	response, err := h.TaskService.List(c.Request.Context())
	if err != nil {
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

// Get godoc
// @Summary      Obtém uma tarefa por ID
// @Tags         tasks
// @Produce      json
// @Param        id path uint64 true "ID da tarefa"
// @Success      200 {object} messages.TaskResponseDTO
// @Failure      404 {object} appErrors.Error
// @Security     BearerAuth
// @Router       /tasks/{id} [get]
func (h *TaskHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ResponseNotFound(c, appErrors.ErrNotFound)
		return
	}

	response, err := h.TaskService.Get(c.Request.Context(), id)
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

// Create godoc
// @Summary      Cria uma nova tarefa
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        request body messages.CreateTaskRequestDTO true "Dados da tarefa"
// @Success      201 {object} messages.TaskResponseDTO
// @Failure      400 {object} appErrors.Error
// @Security     BearerAuth
// @Router       /tasks [post]
func (h *TaskHandler) Create(c *gin.Context) {
	var request messages.CreateTaskRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, appErrors.NewError(err.Error(), nil))
		return
	}

	response, err := h.TaskService.Create(c.Request.Context(), &request)
	if err != nil {
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	ResponseSuccess(c, http.StatusCreated, response)
}

// Update godoc
// @Summary      Atualiza uma tarefa
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        id path uint64 true "ID da tarefa"
// @Param        request body messages.UpdateTaskRequestDTO true "Dados da tarefa"
// @Success      200 {object} messages.TaskResponseDTO
// @Failure      400 {object} appErrors.Error
// @Failure      404 {object} appErrors.Error
// @Security     BearerAuth
// @Router       /tasks/{id} [put]
func (h *TaskHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ResponseNotFound(c, appErrors.ErrNotFound)
		return
	}

	var request messages.UpdateTaskRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, appErrors.NewError(err.Error(), nil))
		return
	}

	response, err := h.TaskService.Update(c.Request.Context(), id, &request)
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

// UpdateStatus godoc
// @Summary      Atualiza o status de uma tarefa
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        id path uint64 true "ID da tarefa"
// @Param        request body messages.UpdateTaskStatusRequestDTO true "Novo status"
// @Success      200 {object} messages.TaskResponseDTO
// @Failure      400 {object} appErrors.Error
// @Failure      404 {object} appErrors.Error
// @Security     BearerAuth
// @Router       /tasks/{id}/status [patch]
func (h *TaskHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ResponseNotFound(c, appErrors.ErrNotFound)
		return
	}

	var request messages.UpdateTaskStatusRequestDTO
	if err := ParseRequest(c, &request); err != nil {
		ResponseBadRequest(c, appErrors.NewError(err.Error(), nil))
		return
	}

	response, err := h.TaskService.UpdateStatus(c.Request.Context(), id, &request)
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

// Delete godoc
// @Summary      Remove uma tarefa
// @Tags         tasks
// @Param        id path uint64 true "ID da tarefa"
// @Success      204 "Removida"
// @Failure      404 {object} appErrors.Error
// @Security     BearerAuth
// @Router       /tasks/{id} [delete]
func (h *TaskHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ResponseNotFound(c, appErrors.ErrNotFound)
		return
	}

	if err := h.TaskService.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			ResponseNotFound(c, appErrors.ErrNotFound)
			return
		}
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	c.Status(http.StatusNoContent)
}
