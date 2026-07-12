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

func (h *TaskHandler) List(c *gin.Context) {
	response, err := h.TaskService.List(c.Request.Context())
	if err != nil {
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

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
