package services

import (
	"context"
	"errors"
	"slices"
	"strings"

	appErrors "github.com/scienceandcode/nucleus-api/internal/app/errors"
	"github.com/scienceandcode/nucleus-api/internal/app/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/app/mappers"
	"github.com/scienceandcode/nucleus-api/internal/app/messages"
	taskEntities "github.com/scienceandcode/nucleus-api/internal/domain/task/entities"
	taskInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/task/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/common"
)

// TaskService is the reference resource service. Every operation is scoped to
// the authenticated user taken from the request context.
type TaskService struct {
	*BaseService
	taskRepository taskInterfaces.TaskRepositoryInterface
}

func NewTaskService(
	baseService *BaseService,
	taskRepository taskInterfaces.TaskRepositoryInterface,
) interfaces.TaskServiceInterface {
	return &TaskService{
		BaseService:    baseService,
		taskRepository: taskRepository,
	}
}

func (s *TaskService) List(ctx context.Context) ([]*messages.TaskResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)

	tasks, err := s.taskRepository.FindAllByUser(ctx, userID)
	if err != nil {
		return nil, appErrors.InternalError
	}

	return mappers.MapTasksToResponseDTOs(tasks), nil
}

func (s *TaskService) Get(ctx context.Context, id uint64) (*messages.TaskResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)

	task, err := s.taskRepository.FindByIDAndUser(ctx, id, userID)
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.InternalError
	}

	return mappers.MapTaskToResponseDTO(task), nil
}

func (s *TaskService) Create(ctx context.Context, request *messages.CreateTaskRequestDTO) (*messages.TaskResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)

	title := strings.TrimSpace(request.Title)
	if fieldErrors := validateTaskTitle(title); fieldErrors != nil {
		return nil, appErrors.NewError("Erro ao criar tarefa.", fieldErrors)
	}

	task, err := s.taskRepository.Create(ctx, &taskEntities.Task{
		UserID:      userID,
		Title:       title,
		Description: strings.TrimSpace(request.Description),
		Status:      taskEntities.TaskStatusPending,
		DueDate:     request.DueDate,
	})
	if err != nil {
		return nil, appErrors.InternalError
	}

	return mappers.MapTaskToResponseDTO(task), nil
}

func (s *TaskService) Update(ctx context.Context, id uint64, request *messages.UpdateTaskRequestDTO) (*messages.TaskResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)

	title := strings.TrimSpace(request.Title)
	if fieldErrors := validateTaskTitle(title); fieldErrors != nil {
		return nil, appErrors.NewError("Erro ao atualizar tarefa.", fieldErrors)
	}

	task, err := s.taskRepository.FindByIDAndUser(ctx, id, userID)
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.InternalError
	}

	task.Title = title
	task.Description = strings.TrimSpace(request.Description)
	task.DueDate = request.DueDate

	updated, err := s.taskRepository.Update(ctx, task)
	if err != nil {
		return nil, appErrors.InternalError
	}

	return mappers.MapTaskToResponseDTO(updated), nil
}

func (s *TaskService) UpdateStatus(ctx context.Context, id uint64, request *messages.UpdateTaskStatusRequestDTO) (*messages.TaskResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)

	status := taskEntities.TaskStatus(strings.ToUpper(strings.TrimSpace(request.Status)))
	if !slices.Contains(taskEntities.AllowedTaskStatuses, status) {
		return nil, appErrors.NewError("Erro ao atualizar status da tarefa.", []*appErrors.FieldError{
			appErrors.NewFieldError("status", "O status informado não é válido."),
		})
	}

	task, err := s.taskRepository.FindByIDAndUser(ctx, id, userID)
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.InternalError
	}

	task.Status = status

	updated, err := s.taskRepository.Update(ctx, task)
	if err != nil {
		return nil, appErrors.InternalError
	}

	return mappers.MapTaskToResponseDTO(updated), nil
}

func (s *TaskService) Delete(ctx context.Context, id uint64) error {
	userID := common.ExtractUserIdFromContext(ctx)
	return s.taskRepository.Delete(ctx, id, userID)
}

func validateTaskTitle(title string) []*appErrors.FieldError {
	if len(title) < 3 {
		return []*appErrors.FieldError{
			appErrors.NewFieldError("title", "O título deve conter pelo menos 3 caracteres."),
		}
	}
	return nil
}
