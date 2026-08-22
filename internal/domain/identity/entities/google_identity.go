package entities

import "time"

const ProviderGoogle = "google"

type GoogleIdentity struct {
	ID        uint64
	UserID    uint64
	Provider  string
	Subject   string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
