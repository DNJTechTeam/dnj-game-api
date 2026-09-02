package interfaces

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/specialevent/entities"
)

type Repository interface {
	Create(ctx context.Context, event *entities.Event) error
	ListForManager(ctx context.Context, userID uint64, global bool) ([]entities.Event, error)
	FindForManager(ctx context.Context, id string, userID uint64, global, lock bool) (*entities.Event, error)
	Save(ctx context.Context, event *entities.Event) error
	FindVisible(ctx context.Context, target string, now time.Time) (*entities.Event, error)
}
