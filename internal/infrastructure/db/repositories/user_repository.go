package repositories

import (
	"context"
	"errors"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *UserRepository) FindByIDForUpdate(ctx context.Context, id uint64) (*entities.User, error) {
	var user models.User
	err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, id).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapUserToEntity(&user), nil
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

func (r *UserRepository) FindByDocumentHash(ctx context.Context, documentHash string) (*entities.User, error) {
	user, err := r.BaseRepository.FindOneBy(ctx, map[string]interface{}{"document_hash": documentHash})
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
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

func (r *UserRepository) RankPosition(ctx context.Context, userID uint64, points int) (int64, error) {
	var ahead int64
	err := r.getDB(ctx).Model(&models.User{}).
		Where("points > ? OR (points = ? AND id < ?)", points, points, userID).Count(&ahead).Error
	if err != nil {
		return 0, handleRepositoryError(err)
	}
	return ahead + 1, nil
}

func (r *UserRepository) ListByRole(ctx context.Context, role entities.UserRole, page uint64) (*messages.PaginatedResponse[entities.User], error) {
	const limit = 20
	var rows []models.User
	err := r.getDB(ctx).Where("role = ?", string(role)).Order("name ASC").Order("id ASC").Limit(limit + 1).Offset(int(page) * limit).Find(&rows).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}
	items := make([]entities.User, len(rows))
	for index := range rows {
		items[index] = *mappers.MapUserToEntity(&rows[index])
	}
	return &messages.PaginatedResponse[entities.User]{Data: items, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: limit}}, nil
}

func (r *UserRepository) UpdateRole(ctx context.Context, userID uint64, role entities.UserRole) error {
	result := r.getDB(ctx).Model(&models.User{}).Where("id = ?", userID).Update("role", string(role))
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return appErrors.ErrNotFound
	}
	return nil
}
