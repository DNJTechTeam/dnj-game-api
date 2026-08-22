package interfaces

import (
	"context"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type GroupInviteServiceInterface interface {
	Create(ctx context.Context, groupID uint64) (*messages.GroupInviteResponseDTO, error)
	Renew(ctx context.Context, groupID uint64, inviteID uint64) (*messages.GroupInviteResponseDTO, error)
	Revoke(ctx context.Context, groupID uint64, inviteID uint64) error
	List(ctx context.Context, groupID uint64, filter *messages.ListGroupInvitesFilterDTO) (*messages.PaginatedResponse[messages.GroupInviteResponseDTO], error)
	Consume(ctx context.Context, request *messages.ConsumeGroupInviteRequestDTO) (*messages.CurrentGroupResponseDTO, error)
}
