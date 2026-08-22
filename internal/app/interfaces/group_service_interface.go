package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type GroupServiceInterface interface {
	Search(ctx context.Context, query string, filter *messages.ListGroupsFilterDTO) (*messages.PaginatedResponse[messages.GroupSummaryDTO], error)
	Current(ctx context.Context) (*messages.CurrentGroupResponseDTO, error)
	UpdateCurrent(ctx context.Context, request *messages.UpdateCurrentGroupRequestDTO) (*messages.CurrentProfileResponseDTO, error)
	Members(ctx context.Context, filter *messages.ListGroupMembersFilterDTO) (*messages.PaginatedResponse[messages.GroupMemberResponseDTO], error)
}
