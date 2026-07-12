package repositories

import (
	"context"

	appErrors "github.com/scienceandcode/nucleus-api/internal/app/errors"
	taskEntities "github.com/scienceandcode/nucleus-api/internal/domain/task/entities"
	taskInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/task/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/mappers"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/models"

	"gorm.io/gorm"
)

// TaskRepository embeds BaseRepository for the trivial CRUD helpers and adds
// owner-scoped queries on top.
type TaskRepository struct {
	*BaseRepository[models.Task]
}

func NewTaskRepository(db *gorm.DB) taskInterfaces.TaskRepositoryInterface {
	return &TaskRepository{
		BaseRepository: NewBaseRepository[models.Task](db),
	}
}

func (r *TaskRepository) Create(ctx context.Context, task *taskEntities.Task) (*taskEntities.Task, error) {
	model := mappers.MapTaskEntityToModel(task)
	if err := r.BaseRepository.Create(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapTaskToEntity(model), nil
}

func (r *TaskRepository) FindByIDAndUser(ctx context.Context, id uint64, userID uint64) (*taskEntities.Task, error) {
	model, err := r.BaseRepository.FindOneBy(ctx, map[string]interface{}{"id": id, "user_id": userID})
	if err != nil {
		return nil, err
	}
	return mappers.MapTaskToEntity(model), nil
}

func (r *TaskRepository) FindAllByUser(ctx context.Context, userID uint64) ([]*taskEntities.Task, error) {
	var taskModels []models.Task
	err := r.getDB(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&taskModels).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}

	tasks := make([]*taskEntities.Task, len(taskModels))
	for i := range taskModels {
		tasks[i] = mappers.MapTaskToEntity(&taskModels[i])
	}
	return tasks, nil
}

func (r *TaskRepository) Update(ctx context.Context, task *taskEntities.Task) (*taskEntities.Task, error) {
	model := mappers.MapTaskEntityToModel(task)
	if err := r.BaseRepository.Update(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapTaskToEntity(model), nil
}

func (r *TaskRepository) Delete(ctx context.Context, id uint64, userID uint64) error {
	result := r.getDB(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&models.Task{})
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected == 0 {
		return appErrors.ErrNotFound
	}
	return nil
}
