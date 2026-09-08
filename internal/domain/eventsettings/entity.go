package entities

import "time"

type EventSettings struct {
	ID            string
	ScoringClosed bool
	ClosedAt      *time.Time
	ClosedBy      *uint64
	AuditNote     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (s *EventSettings) IsScoringClosed() bool {
	return s.ScoringClosed
}
