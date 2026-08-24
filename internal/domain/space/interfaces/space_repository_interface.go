package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
)

type SpaceRepositoryInterface interface {
	List(ctx context.Context, page uint64) (*messages.PaginatedResponse[entities.Space], error)
	Create(ctx context.Context, space *entities.Space) (*entities.Space, error)
	FindByIDForUpdate(ctx context.Context, id string) (*entities.Space, error)
	Update(ctx context.Context, space *entities.Space) (*entities.Space, error)
}
