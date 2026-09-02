package entities

import "time"

type Status string

const (
	StatusDraft  Status = "draft"
	StatusTeaser Status = "teaser"
	StatusActive Status = "active"
	StatusClosed Status = "closed"
)

type Event struct {
	ID              string
	ActivityID      string
	ActivityRunID   *string
	Title           string
	Description     *string
	Points          int
	DurationMinutes int
	Targets         []string
	Status          Status
	TeaserAt        *time.Time
	EndsAt          time.Time
	QRToken         *string
	QRExpiresAt     *time.Time
	CreatedBy       uint64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
