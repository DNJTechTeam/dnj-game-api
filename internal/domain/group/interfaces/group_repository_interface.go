package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
)

type GroupRepositoryInterface interface {
	Create(ctx context.Context, group *entities.Group) (*entities.Group, error)
	FindByID(ctx context.Context, id uint64) (*entities.Group, error)
	// FindByNameExact does a case-insensitive exact match. Returns nil, nil
	// when no group has that name.
	FindByNameExact(ctx context.Context, name string) (*entities.Group, error)
	// Search does a case-insensitive partial match, ordered by name, capped
	// at limit results.
	Search(ctx context.Context, query string, limit int) ([]*entities.Group, error)
	SearchPage(ctx context.Context, query string, page uint64) (*messages.PaginatedResponse[entities.Group], error)
	ExistsByID(ctx context.Context, id uint64) bool
}
