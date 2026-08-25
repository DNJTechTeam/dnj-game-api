package models

import "time"

type GroupInvite struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	GroupID          uint64     `gorm:"not null;index"`
	CodeHash         string     `gorm:"size:64;not null;uniqueIndex"`
	ExpiresAt        time.Time  `gorm:"not null;index"`
	RevokedAt        *time.Time `gorm:"default:null"`
	ConsumedAt       *time.Time `gorm:"default:null"`
	ConsumedByUserID *uint64    `gorm:"default:null"`
	CreatedByUserID  uint64     `gorm:"not null"`
	ReplacesInviteID *uint64    `gorm:"default:null"`
	CreatedAt        time.Time  `gorm:"autoCreateTime:nano"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime:nano"`
}

func (GroupInvite) TableName() string { return "group_invites" }
