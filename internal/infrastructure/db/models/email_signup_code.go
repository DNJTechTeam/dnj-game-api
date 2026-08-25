package models

import "time"

type EmailSignupCode struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement"`
	Email      string     `gorm:"not null;uniqueIndex"`
	CodeHash   string     `gorm:"size:64;not null"`
	ExpiresAt  time.Time  `gorm:"not null"`
	ConsumedAt *time.Time `gorm:"index"`
	Attempts   int        `gorm:"not null;default:0"`
	LastSentAt time.Time  `gorm:"not null"`
	UserID     *uint64    `gorm:"index"`
	CreatedAt  time.Time  `gorm:"autoCreateTime:nano"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime:nano"`
}

func (*EmailSignupCode) TableName() string { return "email_signup_codes" }
