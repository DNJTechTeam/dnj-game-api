package entities

import "time"

// Space is a physical location in the single DNJ installation.
type Space struct {
	ID           string
	Slug         string
	Name         string
	MapReference *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
