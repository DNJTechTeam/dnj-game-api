package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type ContentServiceInterface interface {
	Schedule(ctx context.Context, filter *messages.ListScheduleFilterDTO) (*messages.ScheduleResponseDTO, error)
	ListActivities(ctx context.Context, filter *messages.ListPublicActivitiesFilterDTO) (*messages.PaginatedResponse[messages.PublicActivityResponseDTO], error)
	GetActivity(ctx context.Context, activityID string) (*messages.PublicActivityResponseDTO, error)
}
