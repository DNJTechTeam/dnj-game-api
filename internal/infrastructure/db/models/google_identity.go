package models

import "time"

type GoogleIdentity struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"not null;index"`
	Provider  string    `gorm:"size:32;not null;uniqueIndex:idx_identity_provider_subject"`
	Subject   string    `gorm:"size:255;not null;uniqueIndex:idx_identity_provider_subject"`
	Email     string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime:nano"`
	UpdatedAt time.Time `gorm:"autoUpdateTime:nano"`
}

func (*GoogleIdentity) TableName() string { return "user_identities" }
