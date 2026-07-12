package handlers

import (
	"net/http"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"

	"github.com/gin-gonic/gin"
)

type SubscriptionWebhookHandler struct {
	SubscriptionWebhookService interfaces.SubscriptionWebhookServiceInterface
}

// Ingest godoc
// @Summary      Recebe o webhook de inscrição no evento
// @Description  Grava o payload bruto e cria/atualiza um código de verificação pendente para o inscrito. Payload elástico (formato definido pela plataforma externa).
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Param        X-Webhook-Secret header string true "Secret compartilhado com a plataforma externa"
// @Param        request body object true "Payload do webhook (formato elástico)"
// @Success      204 "Payload recebido"
// @Failure      400 {object} appErrors.Error
// @Failure      401 "Secret inválido"
// @Router       /subscriptions/webhook [post]
func (h *SubscriptionWebhookHandler) Ingest(c *gin.Context) {
	var payload map[string]any
	if err := ParseRequest(c, &payload); err != nil {
		ResponseBadRequest(c, appErrors.NewError(err.Error(), nil))
		return
	}

	if err := h.SubscriptionWebhookService.Ingest(c.Request.Context(), payload); err != nil {
		ResponseBadRequest(c, err.(*appErrors.Error))
		return
	}
	c.Status(http.StatusNoContent)
}
