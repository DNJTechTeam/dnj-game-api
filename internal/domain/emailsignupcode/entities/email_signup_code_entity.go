package entities

import "time"

// EmailSignupCode is a passwordless email signup code: a 6-digit code, sent
// by email, that lets someone create or link a V2 account without Google —
// unlike the V1 verification code, it is self-service and never depends on
// a pre-existing subscription-webhook record.
type EmailSignupCode struct {
	ID         uint64
	Email      string
	CodeHash   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	Attempts   int
	LastSentAt time.Time
	UserID     *uint64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
