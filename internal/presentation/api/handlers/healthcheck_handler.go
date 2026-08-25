package handlers

import (
	"context"
	"net/http"
	"time"

	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthcheckHandler struct {
	DB           *gorm.DB
	MediaStorage mediaInterfaces.Storage
}

// Get godoc
// @Summary      Healthcheck
// @Tags         healthcheck
// @Produce      json
// @Success      200 {object} object
// @Router       /healthcheck [get]
func (h *HealthcheckHandler) Get(c *gin.Context) {
	ResponseSuccess(c, http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthcheckHandler) GetV2(c *gin.Context) {
	ResponseSuccess(c, http.StatusOK, gin.H{"status": "ok", "service": "dnj-game-api"})
}

func (h *HealthcheckHandler) Ready(c *gin.Context) {
	if h.DB == nil {
		ResponseAPIError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Database is unavailable.", nil)
		return
	}

	sqlDB, err := h.DB.DB()
	if err != nil {
		ResponseAPIError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Database is unavailable.", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		ResponseAPIError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Database is unavailable.", nil)
		return
	}
	if h.MediaStorage != nil && h.MediaStorage.ValidateConfiguration() != nil {
		ResponseAPIError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Media configuration is unavailable.", nil)
		return
	}

	ResponseSuccess(c, http.StatusOK, gin.H{"status": "ready"})
}
