package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/identity/entities"
)

type GoogleIdentityRepositoryInterface interface {
	Create(ctx context.Context, identity *entities.GoogleIdentity) (*entities.GoogleIdentity, error)
	FindByProviderAndSubject(ctx context.Context, provider, subject string) (*entities.GoogleIdentity, error)
}
