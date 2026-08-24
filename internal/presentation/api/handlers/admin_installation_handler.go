package handlers

import (
	"net/http"
	"strconv"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/gin-gonic/gin"
)

type AdminInstallationHandler struct {
	AdminInstallationService interfaces.AdminInstallationServiceInterface
}

func parseAdminPage(c *gin.Context) (uint64, error) {
	raw := c.Query("page")
	if raw == "" {
		return 0, nil
	}
	page, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || page == 0 {
		return 0, appErrors.NewAPIServiceError(http.StatusBadRequest, "INVALID_REQUEST", "page deve ser um inteiro maior que zero.", nil)
	}
	return page - 1, nil
}

func adminPageFailure(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	identityFailure(c, err)
	return true
}

func (h *AdminInstallationHandler) ListSpaces(c *gin.Context) {
	page, err := parseAdminPage(c)
	if adminPageFailure(c, err) {
		return
	}
	filter := &messages.ListAdminSpacesFilterDTO{}
	filter.SetPage(page)
	result, err := h.AdminInstallationService.ListSpaces(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *AdminInstallationHandler) CreateSpace(c *gin.Context) {
	request := &messages.CreateAdminSpaceRequestDTO{}
	if err := ParseStrictRequest(c, request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Corpo JSON inválido.", nil)
		return
	}
	result, err := h.AdminInstallationService.CreateSpace(c.Request.Context(), c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusCreated, result)
}

func (h *AdminInstallationHandler) UpdateSpace(c *gin.Context) {
	request := &messages.UpdateAdminSpaceRequestDTO{}
	if err := ParseStrictRequest(c, request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Corpo JSON inválido.", nil)
		return
	}
	result, err := h.AdminInstallationService.UpdateSpace(c.Request.Context(), c.Param("spaceId"), c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *AdminInstallationHandler) ListActivities(c *gin.Context) {
	page, err := parseAdminPage(c)
	if adminPageFailure(c, err) {
		return
	}
	filter := &messages.ListAdminActivitiesFilterDTO{}
	filter.SetPage(page)
	result, err := h.AdminInstallationService.ListActivities(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *AdminInstallationHandler) CreateActivity(c *gin.Context) {
	request := &messages.CreateAdminActivityRequestDTO{}
	if err := ParseStrictRequest(c, request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Corpo JSON inválido.", nil)
		return
	}
	result, err := h.AdminInstallationService.CreateActivity(c.Request.Context(), c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusCreated, result)
}

func (h *AdminInstallationHandler) UpdateActivity(c *gin.Context) {
	request := &messages.UpdateAdminActivityRequestDTO{}
	if err := ParseStrictRequest(c, request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Corpo JSON inválido.", nil)
		return
	}
	result, err := h.AdminInstallationService.UpdateActivity(c.Request.Context(), c.Param("activityId"), c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *AdminInstallationHandler) ListStaff(c *gin.Context) {
	page, err := parseAdminPage(c)
	if adminPageFailure(c, err) {
		return
	}
	filter := &messages.ListAdminStaffFilterDTO{Role: c.Query("role")}
	filter.SetPage(page)
	result, err := h.AdminInstallationService.ListStaff(c.Request.Context(), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *AdminInstallationHandler) UpdateUserRole(c *gin.Context) {
	request := &messages.UpdateAdminUserRoleRequestDTO{}
	if err := ParseStrictRequest(c, request); err != nil {
		ResponseAPIError(c, http.StatusBadRequest, "INVALID_REQUEST", "Corpo JSON inválido.", nil)
		return
	}
	result, err := h.AdminInstallationService.UpdateUserRole(c.Request.Context(), c.Param("userId"), c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *AdminInstallationHandler) ListManagers(c *gin.Context) {
	page, err := parseAdminPage(c)
	if adminPageFailure(c, err) {
		return
	}
	filter := &messages.ListAdminManagersFilterDTO{}
	filter.SetPage(page)
	result, err := h.AdminInstallationService.ListManagers(c.Request.Context(), c.Param("activityId"), filter)
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *AdminInstallationHandler) AssignManager(c *gin.Context) {
	result, err := h.AdminInstallationService.AssignManager(c.Request.Context(), c.Param("activityId"), c.Param("userId"), c.GetHeader("Idempotency-Key"))
	if err != nil {
		identityFailure(c, err)
		return
	}
	ResponseSuccess(c, http.StatusOK, result)
}

func (h *AdminInstallationHandler) RemoveManager(c *gin.Context) {
	if err := h.AdminInstallationService.RemoveManager(c.Request.Context(), c.Param("activityId"), c.Param("userId"), c.GetHeader("Idempotency-Key")); err != nil {
		identityFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
