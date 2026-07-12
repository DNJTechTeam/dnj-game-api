package interfaces

import (
	"context"

	"github.com/scienceandcode/nucleus-api/internal/app/messages"
)

type TaskServiceInterface interface {
	List(ctx context.Context) ([]*messages.TaskResponseDTO, error)
	Get(ctx context.Context, id uint64) (*messages.TaskResponseDTO, error)
	Create(ctx context.Context, request *messages.CreateTaskRequestDTO) (*messages.TaskResponseDTO, error)
	Update(ctx context.Context, id uint64, request *messages.UpdateTaskRequestDTO) (*messages.TaskResponseDTO, error)
	UpdateStatus(ctx context.Context, id uint64, request *messages.UpdateTaskStatusRequestDTO) (*messages.TaskResponseDTO, error)
	Delete(ctx context.Context, id uint64) error
}
