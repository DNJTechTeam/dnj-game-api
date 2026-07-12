package repositories

import (
	"context"
	"errors"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"gorm.io/gorm"
)

type UserRepository struct {
	*BaseRepository[models.User]
}

func NewUserRepository(db *gorm.DB) interfaces.UserRepositoryInterface {
	return &UserRepository{
		BaseRepository: NewBaseRepository[models.User](db),
	}
}

func (r *UserRepository) Create(ctx context.Context, user *entities.User) (*entities.User, error) {
	userModel := mappers.MapEntityToUser(user)

	err := r.BaseRepository.Create(ctx, userModel)
	if err != nil {
		return nil, err
	}

	return mappers.MapUserToEntity(userModel), nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uint64) (*entities.User, error) {
	user, err := r.BaseRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mappers.MapUserToEntity(user), nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	user, err := r.BaseRepository.FindOneBy(ctx, map[string]interface{}{"email": email})

	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return mappers.MapUserToEntity(user), nil
}

func (r *UserRepository) Update(ctx context.Context, user *entities.User) (*entities.User, error) {
	userModel := mappers.MapEntityToUser(user)
	err := r.BaseRepository.Update(ctx, userModel)
	if err != nil {
		return nil, err
	}
	return mappers.MapUserToEntity(userModel), nil
}

func (r *UserRepository) ExistsByID(ctx context.Context, id uint64) bool {
	return r.BaseRepository.ExistsBy(ctx, map[string]interface{}{"id": id})
}
