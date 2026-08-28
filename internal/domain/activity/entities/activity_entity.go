package entities

import (
	"time"

	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
)

type Kind string

const (
	KindSchedule    Kind = "schedule"
	KindCheckpoint  Kind = "checkpoint"
	KindChallenge   Kind = "challenge"
	KindCompetitive Kind = "competitive"
	KindLive        Kind = "live"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusActive    Status = "active"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusArchived  Status = "archived"
)

// Activity is a configurable action in the single DNJ installation.
type Activity struct {
	ID              string
	SpaceID         *string
	Slug            string
	Name            string
	Description     *string
	Kind            Kind
	Status          Status
	StartsAt        *time.Time
	EndsAt          *time.Time
	ActualStartedAt *time.Time
	FlexMinutes     int
	CheckInPoints   int
	MomentPoints    int
	CooldownSeconds int
	AllowsMoment    bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ManagerAssignment struct {
	ActivityID string
	UserID     uint64
	CreatedAt  time.Time
}

// PublicActivity is the public projection of an Activity with its optional
// persisted Space. It deliberately carries no assignments or audit metadata.
type PublicActivity struct {
	Activity Activity
	Space    *spaceEntities.Space
}
