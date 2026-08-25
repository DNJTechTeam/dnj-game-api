package models

import (
	"encoding/json"
	"time"
)

type AdminOperation struct {
	ID             string          `gorm:"type:uuid;primaryKey"`
	ActorUserID    uint64          `gorm:"not null;index"`
	IdempotencyKey string          `gorm:"type:uuid;not null"`
	Operation      string          `gorm:"not null"`
	EntityType     string          `gorm:"not null"`
	EntityRef      string          `gorm:"not null"`
	RequestHash    string          `gorm:"not null"`
	HTTPStatus     int             `gorm:"not null"`
	Response       json.RawMessage `gorm:"type:jsonb;not null"`
	CreatedAt      time.Time       `gorm:"autoCreateTime:nano"`
}

func (*AdminOperation) TableName() string { return "admin_operations" }
