package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/entities"
)

type AdminOperationRepositoryInterface interface {
	FindByActorAndIdempotencyKey(ctx context.Context, actorUserID uint64, key string) (*entities.AdminOperation, error)
	Create(ctx context.Context, operation *entities.AdminOperation) (*entities.AdminOperation, error)
}
