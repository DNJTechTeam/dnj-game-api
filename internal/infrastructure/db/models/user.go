package models

import (
	"time"
)

type User struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Email       string    `gorm:"uniqueIndex;not null"`
	Name        string    `gorm:"not null"`
	MobilePhone string    `gorm:"default:null"`
	Document    string    `gorm:"default:null"`
	Role        string    `gorm:"not null;default:'DEFAULT'"`
	GroupID     *uint64   `gorm:"index;default:null"`
	CreatedAt   time.Time `gorm:"autoCreateTime:nano"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime:nano"`
}

func (user *User) TableName() string {
	return "users"
}
