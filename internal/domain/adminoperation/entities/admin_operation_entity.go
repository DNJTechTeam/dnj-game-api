package entities

import (
	"encoding/json"
	"time"
)

// AdminOperation stores the original safe result of an idempotent
// administrative write. Audit metadata remains deliberately minimal.
type AdminOperation struct {
	ID             string
	ActorUserID    uint64
	IdempotencyKey string
	Operation      string
	EntityType     string
	EntityRef      string
	RequestHash    string
	HTTPStatus     int
	Response       json.RawMessage
	CreatedAt      time.Time
}
