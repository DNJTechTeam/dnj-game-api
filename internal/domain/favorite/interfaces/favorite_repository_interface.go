package interfaces

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
)

type FavoriteRepositoryInterface interface {
	ListVisible(ctx context.Context, userID uint64, generatedAt time.Time, page uint64) (*messages.PaginatedResponse[activityEntities.PublicActivity], error)
	Create(ctx context.Context, favorite *entities.Favorite) (bool, error)
	Delete(ctx context.Context, userID uint64, activityID string) (bool, error)
	FindOperation(ctx context.Context, actorUserID uint64, idempotencyKey string) (*entities.ParticipantOperation, error)
	CreateOperation(ctx context.Context, operation *entities.ParticipantOperation) (*entities.ParticipantOperation, error)
}
