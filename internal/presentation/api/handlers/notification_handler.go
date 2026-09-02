package handlers

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	NotificationService interfaces.NotificationServiceInterface
}

func (h *NotificationHandler) QueueCalled(c *gin.Context) {
	expected := os.Getenv("DNJ_QUEUE_BRIDGE_TOKEN")
	provided := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if expected == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		ResponseAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Credencial da fila inválida.", nil)
		return
	}
	var request messages.QueueCalledNotificationRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie queueId, entryId, participantUserId e calledAt.", nil)
		return
	}
	expectedKey := "queue-called:" + request.QueueID + ":" + request.EntryID
	if c.GetHeader("Idempotency-Key") != expectedKey {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key da fila inválida.", nil)
		return
	}
	response, err := h.NotificationService.CreateQueueCalled(c.Request.Context(), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	status := http.StatusOK
	if response.Created {
		status = http.StatusCreated
	}
	ResponseSuccess(c, status, response)
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

func (h *NotificationHandler) GetPushConfig(c *gin.Context) {
	response, err := h.NotificationService.GetPushConfig(c.Request.Context())
	if err != nil {
		identityFailure(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *NotificationHandler) UpsertPushSubscription(c *gin.Context) {
	var request messages.UpsertPushSubscriptionRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie endpoint, p256dh e auth.", nil)
		return
	}
	response, err := h.NotificationService.UpsertPushSubscription(c.Request.Context(), c.GetHeader("Idempotency-Key"), &request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, response)
}

func (h *NotificationHandler) DeactivatePushSubscription(c *gin.Context) {
	var request messages.DeactivatePushSubscriptionRequestDTO
	if err := ParseStrictRequest(c, &request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Envie endpoint.", nil)
		return
	}
	if err := h.NotificationService.DeactivatePushSubscription(c.Request.Context(), c.GetHeader("Idempotency-Key"), &request); err != nil {
		identityFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
