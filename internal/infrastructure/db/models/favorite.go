package models

import "time"

type UserFavorite struct {
	UserID     uint64    `gorm:"primaryKey"`
	ActivityID string    `gorm:"type:uuid;primaryKey"`
	CreatedAt  time.Time `gorm:"autoCreateTime:nano"`
}

func (*UserFavorite) TableName() string { return "user_favorites" }

type ParticipantOperation struct {
	ID             string    `gorm:"type:uuid;primaryKey"`
	ActorUserID    uint64    `gorm:"not null"`
	IdempotencyKey string    `gorm:"type:uuid;not null"`
	Operation      string    `gorm:"not null"`
	ActivityID     string    `gorm:"type:uuid;not null"`
	IntentHash     string    `gorm:"size:64;not null"`
	HTTPStatus     int       `gorm:"not null"`
	CreatedAt      time.Time `gorm:"autoCreateTime:nano"`
}

func (*ParticipantOperation) TableName() string { return "participant_operations" }
