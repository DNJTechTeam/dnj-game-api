package models

import "time"

type RefreshSession struct {
	ID              string     `gorm:"type:varchar(36);primaryKey"`
	UserID          uint64     `gorm:"not null;index"`
	FamilyID        string     `gorm:"type:varchar(36);not null;index"`
	TokenHash       string     `gorm:"size:64;not null;uniqueIndex"`
	ReplacedByHash  string     `gorm:"size:64;not null;default:''"`
	ExpiresAt       time.Time  `gorm:"not null;index"`
	RevokedAt       *time.Time `gorm:"index"`
	ReuseDetectedAt *time.Time
	CreatedAt       time.Time `gorm:"autoCreateTime:nano"`
	LastUsedAt      time.Time `gorm:"not null"`
}

func (*RefreshSession) TableName() string { return "refresh_sessions" }
