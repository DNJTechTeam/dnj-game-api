package messages

import "time"

type ScoringStatusResponseDTO struct {
	ScoringClosed bool       `json:"scoringClosed"`
	ClosedAt      *time.Time `json:"closedAt,omitempty"`
	AuditNote     string     `json:"auditNote,omitempty"`
}

type CloseScoringRequestDTO struct {
	AuditNote string `json:"auditNote"`
}

type OpenScoringRequestDTO struct {
	AuditNote string `json:"auditNote"`
}
