package interfaces

import (
	"context"

	"github.com/scienceandcode/nucleus-api/internal/domain/task/entities"
)

type TaskRepositoryInterface interface {
	Create(ctx context.Context, task *entities.Task) (*entities.Task, error)
	// FindByIDAndUser scopes the lookup to the owner so a user can never read
	// another user's task. Returns appErrors.ErrNotFound when absent.
	FindByIDAndUser(ctx context.Context, id uint64, userID uint64) (*entities.Task, error)
	FindAllByUser(ctx context.Context, userID uint64) ([]*entities.Task, error)
	Update(ctx context.Context, task *entities.Task) (*entities.Task, error)
	// Delete removes the task only when it belongs to userID. Returns
	// appErrors.ErrNotFound when nothing was deleted.
	Delete(ctx context.Context, id uint64, userID uint64) error
}
