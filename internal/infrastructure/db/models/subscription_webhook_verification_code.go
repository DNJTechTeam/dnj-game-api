package models

import "time"

type SubscriptionWebhookVerificationCode struct {
	ID                    uint64    `gorm:"primaryKey;autoIncrement"`
	SubscriptionWebhookID uint64    `gorm:"not null;index"`
	Email                 string    `gorm:"index;not null"`
	Name                  string    `gorm:"not null"`
	MobilePhone           string    `gorm:"default:null"`
	Document              string    `gorm:"uniqueIndex;not null"`
	VerificationCode      string    `gorm:"not null"`
	Group                 string    `gorm:"default:null"`
	UserID                *uint64   `gorm:"index;default:null"`
	CreatedAt             time.Time `gorm:"autoCreateTime:nano"`
}

func (SubscriptionWebhookVerificationCode) TableName() string {
	return "subscription_webhook_verification_codes"
}
