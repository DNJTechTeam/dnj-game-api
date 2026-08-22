package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/entities"
)

type RefreshSessionRepositoryInterface interface {
	Create(ctx context.Context, session *entities.RefreshSession) (*entities.RefreshSession, error)
	FindByTokenHashForUpdate(ctx context.Context, tokenHash string) (*entities.RefreshSession, error)
	Update(ctx context.Context, session *entities.RefreshSession) (*entities.RefreshSession, error)
	RevokeFamily(ctx context.Context, familyID string, reuseDetected bool) error
}
