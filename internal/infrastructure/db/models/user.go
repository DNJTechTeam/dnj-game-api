package models

import (
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID                  uint64     `gorm:"primaryKey;autoIncrement"`
	Email               string     `gorm:"uniqueIndex;not null"`
	Name                string     `gorm:"not null"`
	Password            string     `gorm:"not null"`
	MobilePhone         string     `gorm:"default:null"`
	EmailConfirmedAt    *time.Time `gorm:"default:null"`
	PasswordConfirmedAt *time.Time `gorm:"default:null"`
	CreatedAt           time.Time  `gorm:"autoCreateTime:nano"`
	UpdatedAt           time.Time  `gorm:"autoUpdateTime:nano"`
}

func (user *User) TableName() string {
	return "users"
}

func (user *User) BeforeCreate(_ *gorm.DB) error {
	user.EmailConfirmedAt = nil

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)

	return nil
}

func (user *User) BeforeUpdate(_ *gorm.DB) error {
	if user.Password != "" && !strings.HasPrefix(user.Password, "$2a$") {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}

	return nil
}
