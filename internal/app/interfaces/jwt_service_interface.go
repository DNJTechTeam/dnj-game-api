package interfaces

import (
	"context"

	uEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
)

type JwtServiceInterface interface {
	GenerateIdentityToken(ctx context.Context, user *uEntities.User) (string, error)
}
