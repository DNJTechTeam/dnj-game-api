package mappers

import (
	"github.com/scienceandcode/nucleus-api/internal/app/messages"
	"github.com/scienceandcode/nucleus-api/internal/domain/task/entities"
)

func MapTaskToResponseDTO(task *entities.Task) *messages.TaskResponseDTO {
	if task == nil {
		return nil
	}

	return &messages.TaskResponseDTO{
		ID:          messages.Uint64StringFromUint64(task.ID),
		Title:       task.Title,
		Description: task.Description,
		Status:      string(task.Status),
		DueDate:     task.DueDate,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
}

func MapTasksToResponseDTOs(tasks []*entities.Task) []*messages.TaskResponseDTO {
	result := make([]*messages.TaskResponseDTO, len(tasks))
	for i, task := range tasks {
		result[i] = MapTaskToResponseDTO(task)
	}
	return result
}
