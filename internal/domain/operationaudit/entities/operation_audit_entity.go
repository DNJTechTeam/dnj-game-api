package entities

import (
	"encoding/json"
	"time"
)

type OperationAudit struct {
	ID             string
	ActorUserID    *uint64
	Action         string
	EntityType     string
	EntityID       *string
	Metadata       json.RawMessage
	IdempotencyKey string
	CreatedAt      time.Time
}
