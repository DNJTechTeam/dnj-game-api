package entities

import "time"

type RefreshSession struct {
	ID              string
	UserID          uint64
	FamilyID        string
	TokenHash       string
	ReplacedByHash  string
	ExpiresAt       time.Time
	RevokedAt       *time.Time
	ReuseDetectedAt *time.Time
	CreatedAt       time.Time
	LastUsedAt      time.Time
}
