package repositories

import (
	"context"
	"errors"

	appErrors "github.com/scienceandcode/nucleus-api/internal/app/errors"
	"github.com/scienceandcode/nucleus-api/internal/app/messages"
	dbPkg "github.com/scienceandcode/nucleus-api/internal/infrastructure/db"

	"gorm.io/gorm"
)

type BaseRepository[T any] struct {
	DB *gorm.DB
}

func handleRepositoryError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appErrors.ErrNotFound
	}

	return appErrors.InternalError
}

func NewBaseRepository[T any](db *gorm.DB) *BaseRepository[T] {
	return &BaseRepository[T]{DB: db}
}

func (r *BaseRepository[T]) getDB(ctx context.Context) *gorm.DB {
	tx := dbPkg.GetTransactionFromContext(ctx)
	if tx != nil {
		return tx
	}
	return r.DB
}

func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	result := r.getDB(ctx).Create(entity)
	return handleRepositoryError(result.Error)
}

func (r *BaseRepository[T]) FindByID(ctx context.Context, id any) (*T, error) {
	var entity T
	result := r.getDB(ctx).First(&entity, id)
	if result.Error != nil {
		return nil, handleRepositoryError(result.Error)
	}
	return &entity, nil
}

func (r *BaseRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	var entities []T
	result := r.getDB(ctx).Find(&entities)
	return entities, handleRepositoryError(result.Error)
}

func (r *BaseRepository[T]) Update(ctx context.Context, entity *T) error {
	result := r.getDB(ctx).Save(entity)
	return handleRepositoryError(result.Error)
}

func (r *BaseRepository[T]) Delete(ctx context.Context, id any) error {
	result := r.getDB(ctx).Delete(new(T), id)
	return handleRepositoryError(result.Error)
}

func (r *BaseRepository[T]) DeleteBy(ctx context.Context, condition map[string]any) error {
	result := r.getDB(ctx).Where(condition).Delete(new(T))
	return handleRepositoryError(result.Error)
}

func (r *BaseRepository[T]) Count(ctx context.Context) (int64, error) {
	var count int64
	result := r.getDB(ctx).Model(new(T)).Count(&count)
	return count, handleRepositoryError(result.Error)
}

func (r *BaseRepository[T]) FindOneBy(ctx context.Context, condition map[string]any) (*T, error) {
	var entity T
	result := r.getDB(ctx).Where(condition).First(&entity)
	if result.Error != nil {
		return nil, handleRepositoryError(result.Error)
	}
	return &entity, nil
}

func (r *BaseRepository[T]) FindAllBy(ctx context.Context, condition map[string]any) ([]T, error) {
	var entities []T
	result := r.getDB(ctx).Where(condition).Find(&entities)
	return entities, handleRepositoryError(result.Error)
}

func (r *BaseRepository[T]) ExistsBy(ctx context.Context, condition map[string]any) bool {
	var count int64
	result := r.getDB(ctx).Model(new(T)).Where(condition).Count(&count)

	if result.Error != nil {
		return false
	}

	return count > 0
}

func (r *BaseRepository[T]) PaginateWithQuery(
	ctx context.Context,
	getDB func(context.Context) *gorm.DB,
	queryBuilder func(*gorm.DB) *gorm.DB,
	page uint64,
) (*messages.PaginatedResponse[T], error) {
	const defaultLimit = 10

	db := getDB(ctx)
	baseQuery := queryBuilder(db.Model(new(T)))

	currentPage := page + 1

	offset := int(page) * defaultLimit

	query := baseQuery

	query = query.Order("id ASC").Limit(defaultLimit + 1).Offset(offset)

	var entities []T
	err := query.Find(&entities).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &messages.PaginatedResponse[T]{
				Data: []T{},
				Pagination: messages.Pagination{
					CurrentPage: messages.Uint64StringFromUint64(currentPage),
					HasNextPage: false,
					Limit:       defaultLimit,
				},
			}, nil
		}
		return nil, handleRepositoryError(err)
	}

	hasNextPage := len(entities) > defaultLimit
	if hasNextPage {
		entities = entities[:defaultLimit]
	}

	return &messages.PaginatedResponse[T]{
		Data: entities,
		Pagination: messages.Pagination{
			CurrentPage: messages.Uint64StringFromUint64(currentPage),
			HasNextPage: hasNextPage,
			Limit:       defaultLimit,
		},
	}, nil
}
