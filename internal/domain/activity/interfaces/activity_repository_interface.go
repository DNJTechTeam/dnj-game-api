package interfaces

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
)

type ActivityRepositoryInterface interface {
	FindAuthorizedForUpdate(ctx context.Context, activityID string, actorUserID uint64, global bool) (*entities.Activity, error)
	TransitionStatus(ctx context.Context, activityID string, from, to entities.Status, updatedAt time.Time) error
}
