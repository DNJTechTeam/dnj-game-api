package interfaces

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/entities"
)

type GroupInviteRepositoryInterface interface {
	Create(ctx context.Context, invite *entities.GroupInvite) (*entities.GroupInvite, error)
	FindByIDAndGroup(ctx context.Context, inviteID uint64, groupID uint64) (*entities.GroupInvite, error)
	FindByHash(ctx context.Context, codeHash string) (*entities.GroupInvite, error)
	ConsumeAvailable(ctx context.Context, inviteID uint64, userID uint64, now time.Time) (bool, error)
	RevokeAvailable(ctx context.Context, inviteID uint64, groupID uint64, now time.Time) (bool, error)
	ListByGroup(ctx context.Context, groupID uint64, page uint64) (*messages.PaginatedResponse[entities.GroupInvite], error)
}
