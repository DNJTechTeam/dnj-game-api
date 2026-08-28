package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID                 uint64         `gorm:"primaryKey;autoIncrement"`
	Email              string         `gorm:"uniqueIndex;not null"`
	Name               string         `gorm:"not null"`
	MobilePhone        string         `gorm:"default:null"`
	Document           string         `gorm:"default:null"`
	DocumentHash       string         `gorm:"size:64;default:null"`
	DocumentLast4      string         `gorm:"size:4;default:null"`
	Role               string         `gorm:"not null;default:'DEFAULT'"`
	ManagerScope       *string        `gorm:"size:32;default:null"`
	GroupID            *uint64        `gorm:"index;default:null"`
	Points             int            `gorm:"not null;default:0"`
	OnboardingComplete bool           `gorm:"not null;default:false"`
	CreatedAt          time.Time      `gorm:"autoCreateTime:nano"`
	UpdatedAt          time.Time      `gorm:"autoUpdateTime:nano"`
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (user *User) TableName() string {
	return "users"
}
