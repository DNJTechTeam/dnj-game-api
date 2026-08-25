package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type FavoriteServiceInterface interface {
	List(ctx context.Context, filter *messages.ListFavoritesFilterDTO) (*messages.PaginatedResponse[messages.PublicActivityResponseDTO], error)
	Put(ctx context.Context, activityID, idempotencyKey string) error
	Delete(ctx context.Context, activityID, idempotencyKey string) error
}
