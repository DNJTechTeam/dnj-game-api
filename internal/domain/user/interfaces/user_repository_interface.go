package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
)

type UserRepositoryInterface interface {
	Create(ctx context.Context, user *entities.User) (*entities.User, error)
	FindByID(ctx context.Context, id uint64) (*entities.User, error)
	FindByIDForUpdate(ctx context.Context, id uint64) (*entities.User, error)
	FindByEmail(ctx context.Context, email string) (*entities.User, error)
	FindByDocumentHash(ctx context.Context, documentHash string) (*entities.User, error)
	Update(ctx context.Context, user *entities.User) (*entities.User, error)
	ExistsByID(ctx context.Context, id uint64) bool
	RankPosition(ctx context.Context, userID uint64, points int) (int64, error)
	ListByRole(ctx context.Context, roles []entities.UserRole, page uint64) (*messages.PaginatedResponse[entities.User], error)
	UpdateRole(ctx context.Context, userID uint64, role entities.UserRole) error
}
