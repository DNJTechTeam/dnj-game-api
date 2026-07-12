package entities

import "time"

// TaskStatus enumerates the lifecycle states of a Task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "PENDING"
	TaskStatusInProgress TaskStatus = "IN_PROGRESS"
	TaskStatusDone       TaskStatus = "DONE"
)

// AllowedTaskStatuses lists every valid TaskStatus, used for validation.
var AllowedTaskStatuses = []TaskStatus{
	TaskStatusPending,
	TaskStatusInProgress,
	TaskStatusDone,
}

// Task is the example CRUD resource shipped with this template. Each task is
// owned by the user that created it (UserID comes from the JWT identity).
type Task struct {
	ID          uint64
	UserID      uint64
	Title       string
	Description string
	Status      TaskStatus
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
