package interfaces

import (
	"context"

	uEntities "github.com/scienceandcode/nucleus-api/internal/domain/user/entities"
)

type JwtServiceInterface interface {
	GenerateIdentityToken(ctx context.Context, user *uEntities.User) (string, error)
}
