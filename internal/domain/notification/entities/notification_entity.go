package entities

import (
	"encoding/json"
	"time"
)

type Category string

const (
	CategoryMomentModeration Category = "moment_moderation"
	CategoryPoints           Category = "points"
	CategoryAnnouncement     Category = "announcement"
)

type State string

const (
	StateUnread State = "unread"
	StateRead   State = "read"
)

// Notification is a persisted, server-derived record of an event relevant to
// a user (Moment moderated, points granted/reversed, administrative
// announcement). Clients never author its type, recipient, content or
// timestamps directly.
type Notification struct {
	ID         string
	UserID     uint64
	Category   Category
	State      State
	Title      string
	Body       string
	SourceType string
	SourceID   *string
	CreatedAt  time.Time
	ReadAt     *time.Time
}

// Preferences controls which derived categories a user receives.
// MomentModeration cannot be disabled — it always reaches the account.
type Preferences struct {
	UserID              uint64
	PointsEnabled       bool
	AnnouncementEnabled bool
	UpdatedAt           time.Time
}

// Operation is one row of the unified idempotency_operations ledger shared
// with every other iteration, scoped here to notification writes.
type Operation struct {
	ID               string
	ActorUserID      uint64
	IdempotencyKey   string
	Operation        string
	ResourceRef      *string
	IntentHash       string
	State            string
	ResultRef        *string
	ResultCount      *int
	ResponseSnapshot json.RawMessage
	HTTPStatus       int
	CreatedAt        time.Time
	CompletedAt      *time.Time
}
