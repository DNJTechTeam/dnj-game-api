package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type GameHandler struct {
	GameService interfaces.GameServiceInterface
}

func requirePublishedQuery(c *gin.Context, allowed ...string) bool {
	return validatePublishedQuery(c, allowed...)
}

func requireEmptyBody(c *gin.Context) bool {
	decoder := json.NewDecoder(c.Request.Body)
	var value any
	err := decoder.Decode(&value)
	if err == io.EOF {
		return true
	}
	if err == nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Esta operação não aceita corpo.", nil)
		return false
	}
	if strings.TrimSpace(c.GetHeader("Content-Length")) == "0" {
		return true
	}
	ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Corpo inválido.", nil)
	return false
}

func (h *GameHandler) List(c *gin.Context) {
	if !requirePublishedQuery(c, "page") {
		return
	}
	page, ok := parsePublicPage(c)
	if !ok {
		return
	}
	filter := &messages.ListGamesFilterDTO{}
	filter.SetPage(page)
	response, err := h.GameService.ListGames(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) Get(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	response, err := h.GameService.GetGame(c.Request.Context(), c.Param("gameId"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) Rankings(c *gin.Context) {
	if !requirePublishedQuery(c, "scope", "page") {
		return
	}
	page, ok := parsePublicPage(c)
	if !ok {
		return
	}
	response, err := h.GameService.Rankings(c.Request.Context(), c.Query("scope"), page)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) Overview(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	response, err := h.GameService.Overview(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) CurrentRun(c *gin.Context) {
	if !requirePublishedQuery(c, "runId") {
		return
	}
	response, err := h.GameService.CurrentRun(c.Request.Context(), c.Query("runId"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	if response == nil {
		c.Status(http.StatusNoContent)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) CurrentParticipation(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	response, err := h.GameService.CurrentParticipation(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	if response == nil {
		c.Status(http.StatusNoContent)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) ValidateQR(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var request messages.QRValidateRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente qrToken.", nil)
		return
	}
	request.IdempotencyKey = c.GetHeader("Idempotency-Key")
	response, status, err := h.GameService.ValidateQR(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, status, response)
}

func (h *GameHandler) ManagerOverview(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	response, err := h.GameService.ManagerOverview(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) ManagerRun(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	response, err := h.GameService.ManagerRun(c.Request.Context(), c.Param("runId"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) AdminCheckpointQR(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	if !requirePublishedQuery(c) {
		return
	}
	response, err := h.GameService.AdminCheckpointQR(c.Request.Context(), c.Param("activityId"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	if response == nil {
		c.Status(http.StatusNoContent)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) CreateManagerGame(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var request messages.CreateManagerGameRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente name.", nil)
		return
	}
	response, status, err := h.GameService.CreateManagerGame(c.Request.Context(), c.GetHeader("Idempotency-Key"), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, status, response)
}

func (h *GameHandler) UpdateManagerGame(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var request messages.UpdateManagerGameRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente name.", nil)
		return
	}
	response, err := h.GameService.UpdateManagerGame(c.Request.Context(), c.Param("gameId"), c.GetHeader("Idempotency-Key"), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) CreateRun(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var request messages.CreateRunRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente gameId.", nil)
		return
	}
	response, status, err := h.GameService.CreateRun(c.Request.Context(), c.GetHeader("Idempotency-Key"), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, status, response)
}

func (h *GameHandler) RotateQR(c *gin.Context) {
	if !requirePublishedQuery(c) || !requireEmptyBody(c) {
		return
	}
	response, status, err := h.GameService.RotateQR(c.Request.Context(), c.Param("runId"), c.GetHeader("Idempotency-Key"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, status, response)
}

func (h *GameHandler) StartRun(c *gin.Context)  { h.transition(c, h.GameService.StartRun) }
func (h *GameHandler) PauseRun(c *gin.Context)  { h.transition(c, h.GameService.PauseRun) }
func (h *GameHandler) ResumeRun(c *gin.Context) { h.transition(c, h.GameService.ResumeRun) }
func (h *GameHandler) CancelRun(c *gin.Context) { h.transition(c, h.GameService.CancelRun) }

func (h *GameHandler) transition(c *gin.Context, operation func(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error)) {
	if !requirePublishedQuery(c) || !requireEmptyBody(c) {
		return
	}
	response, err := operation(c.Request.Context(), c.Param("runId"), c.GetHeader("Idempotency-Key"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *GameHandler) FinalizeRun(c *gin.Context) {
	if !requirePublishedQuery(c) {
		return
	}
	var request messages.FinalizeRunResultsRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie somente results.", nil)
		return
	}
	response, err := h.GameService.FinalizeRun(c.Request.Context(), c.Param("runId"), c.GetHeader("Idempotency-Key"), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}
