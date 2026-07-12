package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type GroupServiceInterface interface {
	Search(ctx context.Context, query string) ([]*messages.GroupSummaryDTO, error)
}
