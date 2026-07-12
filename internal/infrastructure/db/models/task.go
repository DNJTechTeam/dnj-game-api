package models

import "time"

// Task is the GORM model for the example resource. The index on user_id backs
// the owner-scoped queries in TaskRepository.
type Task struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	UserID      uint64     `gorm:"not null;index:idx_tasks_user_id"`
	Title       string     `gorm:"not null"`
	Description string     `gorm:"default:null"`
	Status      string     `gorm:"not null;default:'PENDING'"`
	DueDate     *time.Time `gorm:"default:null"`
	CreatedAt   time.Time  `gorm:"autoCreateTime:nano"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime:nano"`
}

func (Task) TableName() string {
	return "tasks"
}
