package entities

import "time"

type GroupInvite struct {
	ID               uint64
	GroupID          uint64
	CodeHash         string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	ConsumedAt       *time.Time
	ConsumedByUserID *uint64
	CreatedByUserID  uint64
	ReplacesInviteID *uint64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
