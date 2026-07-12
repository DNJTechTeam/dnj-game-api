package handlers

import (
	"net/http"
	"strconv"

	"github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"

	"github.com/gin-gonic/gin"
)

func ParseRequest(c *gin.Context, dto any) error {
	if err := c.BindJSON(&dto); err != nil {
		return err
	}
	return nil
}

func ResponseBadRequest(c *gin.Context, err *errors.Error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err})
}

func ResponseSuccess(c *gin.Context, httpStatusCode int, data any) {
	c.JSON(httpStatusCode, data)
}

func ResponseUnauthorized(c *gin.Context, err error) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": err})
}

func ResponseNotFound(c *gin.Context, err *errors.Error) {
	c.JSON(http.StatusNotFound, gin.H{"error": err})
}

func ResponseForbidden(c *gin.Context, err error) {
	c.JSON(http.StatusForbidden, gin.H{"error": err})
}

func ResponseInternalServerError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": errors.NewSimpleError("An internal server error Occurred. Please try again later.")})
}

func ParsePaginationFromQuery(c *gin.Context, filter messages.PaginationFilterInterface) {
	if pageStr := c.Query("page"); pageStr != "" {
		page, err := strconv.ParseUint(pageStr, 10, 64)
		if err == nil {
			if page > 0 {
				filter.SetPage(page - 1)
			} else {
				filter.SetPage(0)
			}
		}
	}
}
