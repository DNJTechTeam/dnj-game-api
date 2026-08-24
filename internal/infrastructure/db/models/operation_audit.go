package models

import (
	"encoding/json"
	"time"
)

type OperationAudit struct {
	ID             string          `gorm:"type:uuid;primaryKey"`
	ActorUserID    *uint64         `gorm:"index;default:null"`
	Action         string          `gorm:"not null"`
	EntityType     string          `gorm:"not null"`
	EntityID       *string         `gorm:"type:uuid;index;default:null"`
	Metadata       json.RawMessage `gorm:"type:jsonb;not null"`
	IdempotencyKey string          `gorm:"type:uuid;not null"`
	CreatedAt      time.Time       `gorm:"autoCreateTime:nano"`
}

func (*OperationAudit) TableName() string { return "operation_audit" }
