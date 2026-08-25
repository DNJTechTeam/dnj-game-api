package entities

import "time"

// Favorite links one authenticated participant to one public Activity.
type Favorite struct {
	UserID     uint64
	ActivityID string
	CreatedAt  time.Time
}

// ParticipantOperation stores the minimal, body-free result needed to make
// participant writes idempotent per actor and intention.
type ParticipantOperation struct {
	ID             string
	ActorUserID    uint64
	IdempotencyKey string
	Operation      string
	ActivityID     string
	IntentHash     string
	HTTPStatus     int
	ResultRef      *string
	ResultPoints   *int
	CreatedAt      time.Time
}
