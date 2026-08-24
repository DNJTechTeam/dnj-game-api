package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type AdminInstallationServiceInterface interface {
	ListSpaces(ctx context.Context, filter *messages.ListAdminSpacesFilterDTO) (*messages.PaginatedResponse[messages.SpaceResponseDTO], error)
	CreateSpace(ctx context.Context, key string, request *messages.CreateAdminSpaceRequestDTO) (*messages.SpaceResponseDTO, error)
	UpdateSpace(ctx context.Context, spaceID, key string, request *messages.UpdateAdminSpaceRequestDTO) (*messages.SpaceResponseDTO, error)
	ListActivities(ctx context.Context, filter *messages.ListAdminActivitiesFilterDTO) (*messages.PaginatedResponse[messages.AdminActivityResponseDTO], error)
	CreateActivity(ctx context.Context, key string, request *messages.CreateAdminActivityRequestDTO) (*messages.AdminActivityResponseDTO, error)
	UpdateActivity(ctx context.Context, activityID, key string, request *messages.UpdateAdminActivityRequestDTO) (*messages.AdminActivityResponseDTO, error)
	ListStaff(ctx context.Context, filter *messages.ListAdminStaffFilterDTO) (*messages.PaginatedResponse[messages.AdminStaffResponseDTO], error)
	UpdateUserRole(ctx context.Context, userID, key string, request *messages.UpdateAdminUserRoleRequestDTO) (*messages.AdminUserRoleResponseDTO, error)
	ListManagers(ctx context.Context, activityID string, filter *messages.ListAdminManagersFilterDTO) (*messages.PaginatedResponse[messages.AdminStaffResponseDTO], error)
	AssignManager(ctx context.Context, activityID, userID, key string) (*messages.AdminManagerAssignmentResponseDTO, error)
	RemoveManager(ctx context.Context, activityID, userID, key string) error
}
