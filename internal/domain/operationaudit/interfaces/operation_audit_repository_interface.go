package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/entities"
)

type OperationAuditRepositoryInterface interface {
	FindByActorAndIdempotencyKey(ctx context.Context, actorUserID uint64, key string) (*entities.OperationAudit, error)
	Create(ctx context.Context, audit *entities.OperationAudit) (*entities.OperationAudit, error)
}
