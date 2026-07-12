package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthcheckHandler struct{}

// Get godoc
// @Summary      Healthcheck
// @Tags         healthcheck
// @Produce      json
// @Success      200 {object} object
// @Router       /healthcheck [get]
func (h *HealthcheckHandler) Get(c *gin.Context) {
	ResponseSuccess(c, http.StatusOK, gin.H{"status": "ok"})
}
