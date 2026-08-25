package handlers

import (
	"net/http"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	NotificationService interfaces.NotificationServiceInterface
}

func (h *NotificationHandler) GetPreferences(c *gin.Context) {
	response, err := h.NotificationService.GetPreferences(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *NotificationHandler) UpdatePreferences(c *gin.Context) {
	var request messages.UpdateNotificationPreferencesRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Envie somente pointsEnabled e/ou announcementEnabled.",
			nil,
		)
		return
	}
	response, err := h.NotificationService.UpdatePreferences(c.Request.Context(), c.GetHeader("Idempotency-Key"), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *NotificationHandler) List(c *gin.Context) {
	filter := &messages.ListNotificationsFilterDTO{}
	ParsePaginationFromQuery(c, filter)

	response, err := h.NotificationService.List(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	if !requireEmptyBody(c) {
		return
	}
	response, err := h.NotificationService.MarkRead(
		c.Request.Context(),
		c.Param("notificationId"),
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *NotificationHandler) AdminSend(c *gin.Context) {
	var request messages.AdminSendNotificationRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Envie somente title, body e targetUserIds opcional.",
			nil,
		)
		return
	}
	response, err := h.NotificationService.AdminSend(c.Request.Context(), c.GetHeader("Idempotency-Key"), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusCreated, response)
}
