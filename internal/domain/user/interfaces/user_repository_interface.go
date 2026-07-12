package interfaces

import (
	"context"

	"github.com/scienceandcode/nucleus-api/internal/domain/user/entities"
)

type UserRepositoryInterface interface {
	Create(ctx context.Context, user *entities.User) (*entities.User, error)
	FindByID(ctx context.Context, id uint64) (*entities.User, error)
	FindByEmail(ctx context.Context, email string) (*entities.User, error)
	Update(ctx context.Context, user *entities.User) (*entities.User, error)
	ExistsByID(ctx context.Context, id uint64) bool
}
