package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/entities"
)

type GroupMembershipRepositoryInterface interface {
	UpsertForUser(ctx context.Context, membership *entities.GroupMembership) (*entities.GroupMembership, error)
	DeleteByUser(ctx context.Context, userID uint64) error
	FindByUser(ctx context.Context, userID uint64) (*entities.GroupMembership, error)
	ListMembers(ctx context.Context, groupID uint64, page uint64) (*messages.PaginatedResponse[entities.GroupMember], error)
}
