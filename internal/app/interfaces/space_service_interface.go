package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type SpaceServiceInterface interface {
	List(ctx context.Context, filter *messages.ListSpacesFilterDTO) (*messages.PaginatedResponse[messages.SpaceResponseDTO], error)
}
