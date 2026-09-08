package interfaces

import (
	"context"

	entities "github.com/dnjtechteam/dnj-game-api/internal/domain/eventsettings"
)

type EventSettingsRepositoryInterface interface {
	Get(ctx context.Context) (*entities.EventSettings, error)
	SetScoringClosed(ctx context.Context, closed bool, closedBy *uint64, auditNote string) error
}
