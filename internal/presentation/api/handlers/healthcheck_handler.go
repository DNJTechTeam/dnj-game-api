package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthcheckHandler struct{}

func (h *HealthcheckHandler) Get(c *gin.Context) {
	ResponseSuccess(c, http.StatusOK, gin.H{"status": "ok"})
}
