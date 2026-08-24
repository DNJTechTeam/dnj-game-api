package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
)

type SpaceRepositoryInterface interface {
	List(ctx context.Context, page uint64) (*messages.PaginatedResponse[entities.Space], error)
}
