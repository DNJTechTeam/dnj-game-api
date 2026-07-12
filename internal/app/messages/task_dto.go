package messages

import "time"

type CreateTaskRequestDTO struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"dueDate"`
}

type UpdateTaskRequestDTO struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"dueDate"`
}

type UpdateTaskStatusRequestDTO struct {
	Status string `json:"status"`
}

type TaskResponseDTO struct {
	ID          Uint64String `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	DueDate     *time.Time   `json:"dueDate"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}
