package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type EventSettingsServiceInterface interface {
	GetScoringStatus(ctx context.Context) (*messages.ScoringStatusResponseDTO, error)
	CloseScoring(ctx context.Context, auditNote string) (*messages.ScoringStatusResponseDTO, error)
	OpenScoring(ctx context.Context, auditNote string) (*messages.ScoringStatusResponseDTO, error)
	IsScoringClosed(ctx context.Context) (bool, error)
}
