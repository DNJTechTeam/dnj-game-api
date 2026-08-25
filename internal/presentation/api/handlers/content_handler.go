package handlers

import (
	"net/http"
	"strconv"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type ContentHandler struct {
	ContentService interfaces.ContentServiceInterface
}

func validatePublishedQuery(c *gin.Context, allowed ...string) bool {
	accepted := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		accepted[key] = struct{}{}
	}
	for key, values := range c.Request.URL.Query() {
		if _, ok := accepted[key]; !ok || len(values) != 1 {
			ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Parâmetros de query inválidos.", nil)
			return false
		}
	}
	return true
}

func parsePublicPage(c *gin.Context) (uint64, bool) {
	raw, present := c.GetQuery("page")
	if !present {
		return 0, true
	}
	page, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || page > 1_000_000 {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "page deve ser um inteiro não negativo.", nil)
		return 0, false
	}
	if page == 0 {
		return 0, true
	}
	return page - 1, true
}

func (h *ContentHandler) Schedule(c *gin.Context) {
	if !validatePublishedQuery(c, "view", "sector") {
		return
	}
	result, err := h.ContentService.Schedule(c.Request.Context(), &messages.ListScheduleFilterDTO{View: c.Query("view"), Sector: c.Query("sector")})
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *ContentHandler) ListActivities(c *gin.Context) {
	if !validatePublishedQuery(c, "kind", "spaceId", "page") {
		return
	}
	page, ok := parsePublicPage(c)
	if !ok {
		return
	}
	filter := &messages.ListPublicActivitiesFilterDTO{Kind: c.Query("kind"), SpaceID: c.Query("spaceId")}
	filter.SetPage(page)
	result, err := h.ContentService.ListActivities(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *ContentHandler) GetActivity(c *gin.Context) {
	if !validatePublishedQuery(c) {
		return
	}
	result, err := h.ContentService.GetActivity(c.Request.Context(), c.Param("activityId"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}
