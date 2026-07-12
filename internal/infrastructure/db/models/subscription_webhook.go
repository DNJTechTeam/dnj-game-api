package models

import "time"

type SubscriptionWebhook struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	Payload   string    `gorm:"type:jsonb;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime:nano"`
}

func (SubscriptionWebhook) TableName() string {
	return "subscription_webhooks"
}
